package io3mf

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"sort"
	"sync"

	go3mf "github.com/qmuntal/go3mf"
	"github.com/qmuntal/go3mf/iohelper"
)

// A XMLDecoder is anything that can decode a stream of XML tokens, including a Decoder.
type XMLDecoder interface {
	xml.TokenReader
	// Skip reads tokens until it has consumed the end element matching the most recent start element already consumed.
	Skip() error
	// InputOffset returns the input stream byte offset of the current decoder position.
	InputOffset() int64
}

type relationship interface {
	Type() string
	TargetURI() string
}

type packageFile interface {
	Name() string
	FindFileFromRel(string) (packageFile, bool)
	Relationships() []relationship
	Open() (io.ReadCloser, error)
}

type packageReader interface {
	Open(func(r io.Reader) io.ReadCloser) error
	FindFileFromRel(string) (packageFile, bool)
	FindFileFromName(string) (packageFile, bool)
}

// ReadCloser wrapps a Decoder than can be closed.
type ReadCloser struct {
	f *os.File
	*Decoder
}

// OpenReader will open the 3MF file specified by name and return a ReadCloser.
func OpenReader(name string) (*ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &ReadCloser{f: f, Decoder: NewDecoder(f, fi.Size())}, nil
}

// Close closes the 3MF file, rendering it unusable for I/O.
func (r *ReadCloser) Close() error {
	return r.f.Close()
}

type nodeDecoder interface {
	Open()
	Attributes([]xml.Attr) bool
	Text([]byte) bool
	Child(xml.Name) nodeDecoder
	Close() bool
	SetScanner(*iohelper.Scanner)
}

type emptyDecoder struct {
	scanner *iohelper.Scanner
}

func (d *emptyDecoder) Open()                          { return }
func (d *emptyDecoder) Attributes([]xml.Attr) bool     { return true }
func (d *emptyDecoder) Text([]byte) bool               { return true }
func (d *emptyDecoder) Child(xml.Name) nodeDecoder     { return nil }
func (d *emptyDecoder) Close() bool                    { return true }
func (d *emptyDecoder) SetScanner(s *iohelper.Scanner) { d.scanner = s }

type topLevelDecoder struct {
	emptyDecoder
	model  *go3mf.Model
	isRoot bool
}

func (d *topLevelDecoder) Child(name xml.Name) (child nodeDecoder) {
	modelName := xml.Name{Space: nsCoreSpec, Local: attrModel}
	if name == modelName {
		child = &modelDecoder{model: d.model}
	}
	return
}

// modelFileDecoder cannot be reused between goroutines.
type modelFileDecoder struct {
	scanner *iohelper.Scanner
}

func (d *modelFileDecoder) Decode(ctx context.Context, x XMLDecoder, model *go3mf.Model, path string, isRoot, strict bool) (err error) {
	d.scanner = iohelper.NewScanner(model)
	d.scanner.IsRoot = isRoot
	d.scanner.Strict = strict
	d.scanner.ModelPath = path
	state := make([]nodeDecoder, 0, 10)
	names := make([]xml.Name, 0, 10)

	var (
		currentDecoder nodeDecoder
		tmpDecoder     nodeDecoder
		currentName    xml.Name
		t              xml.Token
	)
	nextBytesCheck := checkEveryBytes
	currentDecoder = &topLevelDecoder{isRoot: isRoot, model: model}
	currentDecoder.SetScanner(d.scanner)

	for {
		t, err = x.Token()
		if err != nil {
			break
		}
		switch tp := t.(type) {
		case xml.StartElement:
			tmpDecoder = currentDecoder.Child(tp.Name)
			if tmpDecoder != nil {
				tmpDecoder.SetScanner(d.scanner)
				state = append(state, currentDecoder)
				names = append(names, currentName)
				currentName = tp.Name
				d.scanner.Element = tp.Name.Local
				currentDecoder = tmpDecoder
				currentDecoder.Open()
				if !currentDecoder.Attributes(tp.Attr) {
					err = d.scanner.Err
				}
			} else {
				err = x.Skip()
			}
		case xml.CharData:
			if !currentDecoder.Text(tp) {
				err = d.scanner.Err
			}
		case xml.EndElement:
			if currentName == tp.Name {
				d.scanner.Element = tp.Name.Local
				if currentDecoder.Close() {
					currentDecoder, state = state[len(state)-1], state[:len(state)-1]
					currentName, names = names[len(names)-1], names[:len(names)-1]
				} else {
					err = d.scanner.Err
				}
			}
			if x.InputOffset() > nextBytesCheck {
				select {
				case <-ctx.Done():
					err = ctx.Err()
				default: // Default is must to avoid blocking
				}
				nextBytesCheck += checkEveryBytes
			}
		}
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return err
}

// Decoder implements a 3mf file decoder.
type Decoder struct {
	Strict              bool
	Warnings            []error
	AttachmentRelations []string
	p                   packageReader
	x                   func(r io.Reader) XMLDecoder
	flate               func(r io.Reader) io.ReadCloser
	productionModels    map[string]packageFile
	ctx                 context.Context
}

// NewDecoder returns a new Decoder reading a 3mf file from r.
func NewDecoder(r io.ReaderAt, size int64) *Decoder {
	return &Decoder{
		p:      &opcReader{ra: r, size: size},
		Strict: true,
	}
}

// Decode reads the 3mf file and unmarshall its content into the model.
func (d *Decoder) Decode(model *go3mf.Model) error {
	return d.DecodeContext(context.Background(), model)
}

// SetXMLDecoder sets the XML decoder to use when reading XML files.
func (d *Decoder) SetXMLDecoder(x func(r io.Reader) XMLDecoder) {
	d.x = x
}

// SetDecompressor sets or overrides a custom decompressor for deflating the zip package.
func (d *Decoder) SetDecompressor(dcomp func(r io.Reader) io.ReadCloser) {
	d.flate = dcomp
}

// DecodeContext reads the 3mf file and unmarshall its content into the model.
func (d *Decoder) DecodeContext(ctx context.Context, model *go3mf.Model) error {
	rootFile, err := d.processOPC(model)
	if err != nil {
		return err
	}
	if err := d.processNonRootModels(ctx, model); err != nil {
		return err
	}
	return d.processRootModel(ctx, rootFile, model)
}

func (d *Decoder) tokenReader(r io.Reader) XMLDecoder {
	if d.x == nil {
		return xml.NewDecoder(r)
	}
	return d.x(r)
}

func (d *Decoder) processRootModel(ctx context.Context, rootFile packageFile, model *go3mf.Model) error {
	f, err := rootFile.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	mf := modelFileDecoder{}
	err = mf.Decode(ctx, d.tokenReader(f), model, rootFile.Name(), true, d.Strict)
	select {
	case <-ctx.Done():
		err = ctx.Err()
	default: // Default is must to avoid blocking
	}
	d.addModelFile(mf.scanner, model)
	return err
}

func (d *Decoder) addModelFile(p *iohelper.Scanner, model *go3mf.Model) {
	model.UUID = p.UUID
	for _, bi := range p.BuildItems {
		model.BuildItems = append(model.BuildItems, bi)
	}
	for _, res := range p.Resources {
		model.Resources = append(model.Resources, res)
	}
	for _, res := range p.Warnings {
		d.Warnings = append(d.Warnings, res)
	}
}

func (d *Decoder) processNonRootModels(ctx context.Context, model *go3mf.Model) (err error) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var files sync.Map
	prodAttCount := len(model.ProductionAttachments)
	wg.Add(prodAttCount)
	for i := 0; i < prodAttCount; i++ {
		go func(i int) {
			defer wg.Done()
			f, err1 := d.readProductionAttachmentModel(ctx, i, model)
			select {
			case <-ctx.Done():
				return // Error somewhere, terminate
			default: // Default is must to avoid blocking
			}
			if err1 != nil {
				err = err1
				cancel()
			}
			files.Store(i, f)
		}(i)
	}
	wg.Wait()
	if err != nil {
		return err
	}
	indices := make([]int, 0, prodAttCount)
	files.Range(func(key, value interface{}) bool {
		indices = append(indices, key.(int))
		return true
	})
	sort.Ints(indices)
	for _, index := range indices {
		f, _ := files.Load(index)
		d.addModelFile(f.(*iohelper.Scanner), model)
	}
	return nil
}

func (d *Decoder) processOPC(model *go3mf.Model) (packageFile, error) {
	err := d.p.Open(d.flate)
	if err != nil {
		return nil, err
	}
	rootFile, ok := d.p.FindFileFromRel(relTypeModel3D)
	if !ok {
		return nil, errors.New("go3mf: package does not have root model")
	}

	model.Path = rootFile.Name()
	d.extractTexturesAttachments(rootFile, model)
	d.extractCustomAttachments(rootFile, model)
	d.extractModelAttachments(rootFile, model)
	for _, a := range model.ProductionAttachments {
		file, _ := d.p.FindFileFromName(a.Path)
		d.extractCustomAttachments(file, model)
		d.extractTexturesAttachments(file, model)
	}
	thumbFile, ok := rootFile.FindFileFromRel(relTypeThumbnail)
	if ok {
		if buff, err := copyFile(thumbFile); err == nil {
			model.SetThumbnail(buff)
		}
	}

	return rootFile, nil
}

func (d *Decoder) extractTexturesAttachments(rootFile packageFile, model *go3mf.Model) {
	for _, rel := range rootFile.Relationships() {
		if rel.Type() != relTypeTexture3D && rel.Type() != relTypeThumbnail {
			continue
		}

		if file, ok := rootFile.FindFileFromRel(rel.Type()); ok {
			model.Attachments = d.addAttachment(model.Attachments, file, rel.Type())
		}
	}
}

func (d *Decoder) extractCustomAttachments(rootFile packageFile, model *go3mf.Model) {
	for _, rel := range d.AttachmentRelations {
		if file, ok := rootFile.FindFileFromRel(rel); ok {
			model.Attachments = d.addAttachment(model.Attachments, file, rel)
		}
	}
}

func (d *Decoder) extractModelAttachments(rootFile packageFile, model *go3mf.Model) {
	d.productionModels = make(map[string]packageFile)
	for _, rel := range rootFile.Relationships() {
		if rel.Type() != relTypeModel3D {
			continue
		}

		if file, ok := rootFile.FindFileFromRel(rel.TargetURI()); ok {
			model.ProductionAttachments = append(model.ProductionAttachments, &go3mf.ProductionAttachment{
				RelationshipType: rel.Type(),
				Path:             file.Name(),
			})
			d.productionModels[file.Name()] = file
		}
	}
}

func (d *Decoder) addAttachment(attachments []*go3mf.Attachment, file packageFile, relType string) []*go3mf.Attachment {
	buff, err := copyFile(file)
	if err == nil {
		return append(attachments, &go3mf.Attachment{
			RelationshipType: relType,
			Path:             file.Name(),
			Stream:           buff,
		})
	}
	return attachments
}

func (d *Decoder) readProductionAttachmentModel(ctx context.Context, i int, model *go3mf.Model) (*iohelper.Scanner, error) {
	attachment := model.ProductionAttachments[i]
	file, err := d.productionModels[attachment.Path].Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	mf := modelFileDecoder{}
	err = mf.Decode(ctx, d.tokenReader(file), model, attachment.Path, false, d.Strict)
	return mf.scanner, err
}

func copyFile(file packageFile) (io.Reader, error) {
	stream, err := file.Open()
	if err != nil {
		return nil, err
	}
	buff := new(bytes.Buffer)
	_, err = io.Copy(buff, stream)
	stream.Close()
	return buff, err
}
