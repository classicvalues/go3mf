package io3mf

import (
	"encoding/xml"
	"strconv"

	"github.com/qmuntal/go3mf"
	"github.com/qmuntal/go3mf/mesh"
)

type beamLatticeDecoder struct {
	r        *Reader
	resource *go3mf.MeshResource
}

func (d *beamLatticeDecoder) Open() error  { return nil }
func (d *beamLatticeDecoder) Close() error { return nil }

func (d *beamLatticeDecoder) Attributes(attrs []xml.Attr) (err error) {
	for _, a := range attrs {
		if a.Name.Space != "" {
			continue
		}
		switch a.Name.Local {
		case attrRadius:
			d.resource.Mesh.DefaultRadius, err = strconv.ParseFloat(a.Value, 64)
		case attrMinLength, attrPrecision: // lib3mf legacy
			d.resource.Mesh.MinLength, err = strconv.ParseFloat(a.Value, 64)
		case attrClippingMode, attrClipping: // lib3mf legacy
			d.resource.BeamLatticeAttributes.ClipMode, _ = newClipMode(a.Value)
		case attrClippingMesh:
			var val uint64
			val, err = strconv.ParseUint(a.Value, 10, 32)
			d.resource.BeamLatticeAttributes.ClippingMeshID = uint32(val)
		case attrRepresentationMesh:
			var val uint64
			val, err = strconv.ParseUint(a.Value, 10, 32)
			d.resource.BeamLatticeAttributes.RepresentationMeshID = uint32(val)
		case attrCap:
			d.resource.Mesh.CapMode, _ = newCapMode(a.Value)
		}
		if err != nil {
			break
		}
	}
	return
}

func (d *beamLatticeDecoder) Child(name xml.Name) (child nodeDecoder) {
	if name.Space == nsBeamLatticeSpec {
		if name.Local == attrBeams {
			child = &beamsDecoder{r: d.r, mesh: d.resource.Mesh}
		} else if name.Local == attrBeamSets {
			child = &beamSetsDecoder{r: d.r, mesh: d.resource.Mesh}
		}
	}
	return
}

type beamsDecoder struct {
	r           *Reader
	mesh        *mesh.Mesh
	beamDecoder beamDecoder
}

func (d *beamsDecoder) Open() error {
	d.beamDecoder.r = d.r
	d.beamDecoder.mesh = d.mesh
	return nil
}
func (d *beamsDecoder) Close() error                            { return nil }
func (d *beamsDecoder) Attributes(attrs []xml.Attr) (err error) { return }

func (d *beamsDecoder) Child(name xml.Name) (child nodeDecoder) {
	if name.Space == nsBeamLatticeSpec && name.Local == attrBeam {
		child = &d.beamDecoder
	}
	return
}

type beamDecoder struct {
	r    *Reader
	mesh *mesh.Mesh
}

func (d *beamDecoder) Open() error                                        { return nil }
func (d *beamDecoder) Close() error                                       { return nil }
func (d *beamDecoder) Child(name xml.Name) (child nodeDecoder) { return }

func (d *beamDecoder) Attributes(attrs []xml.Attr) (err error) {
	var (
		v1, v2           uint64
		r1, r2           float64
		cap1, cap2       mesh.CapMode
		hasCap1, hasCap2 bool
	)
	for _, a := range attrs {
		if a.Name.Space != "" {
			continue
		}
		switch a.Name.Local {
		case attrV1:
			v1, err = strconv.ParseUint(a.Value, 10, 32)
		case attrV2:
			v2, err = strconv.ParseUint(a.Value, 10, 32)
		case attrR1:
			r1, err = strconv.ParseFloat(a.Value, 64)
		case attrR2:
			r2, err = strconv.ParseFloat(a.Value, 64)
		case attrCap1:
			cap1, _ = newCapMode(a.Value)
			hasCap1 = true
		case attrCap2:
			cap2, _ = newCapMode(a.Value)
			hasCap2 = true
		}
		if err != nil {
			break
		}
	}
	if err != nil {
		return
	}
	if r1 == 0 {
		r1 = d.mesh.DefaultRadius
	}
	if r2 == 0 {
		r2 = r1
	}
	if !hasCap1 {
		cap1 = d.mesh.CapMode
	}
	if !hasCap2 {
		cap2 = d.mesh.CapMode
	}
	d.mesh.Beams = append(d.mesh.Beams, mesh.Beam{
		NodeIndices: [2]uint32{uint32(v1), uint32(v2)},
		Radius:      [2]float64{r1, r2},
		CapMode:     [2]mesh.CapMode{cap1, cap2},
	})
	return
}

type beamSetsDecoder struct {
	r    *Reader
	mesh *mesh.Mesh
}

func (d *beamSetsDecoder) Open() error                             { return nil }
func (d *beamSetsDecoder) Close() error                            { return nil }
func (d *beamSetsDecoder) Attributes(attrs []xml.Attr) (err error) { return }

func (d *beamSetsDecoder) Child(name xml.Name) (child nodeDecoder) {
	if name.Space == nsBeamLatticeSpec && name.Local == attrBeamSet {
		child = &beamSetDecoder{r: d.r, mesh: d.mesh}
	}
	return
}

type beamSetDecoder struct {
	r              *Reader
	mesh           *mesh.Mesh
	beamSet        mesh.BeamSet
	beamRefDecoder beamRefDecoder
}

func (d *beamSetDecoder) Open() error {
	d.beamRefDecoder.r = d.r
	d.beamRefDecoder.beamSet = &d.beamSet
	return nil
}

func (d *beamSetDecoder) Close() error {
	d.mesh.BeamSets = append(d.mesh.BeamSets, d.beamSet)
	return nil
}

func (d *beamSetDecoder) Attributes(attrs []xml.Attr) (err error) {
	for _, a := range attrs {
		if a.Name.Space != "" {
			continue
		}
		switch a.Name.Local {
		case attrName:
			d.beamSet.Name = a.Value
		case attrIdentifier:
			d.beamSet.Identifier = a.Value
		}
	}
	return
}

func (d *beamSetDecoder) Child(name xml.Name) (child nodeDecoder) {
	if name.Space == nsBeamLatticeSpec && name.Local == attrRef {
		child = &d.beamRefDecoder
	}
	return
}

type beamRefDecoder struct {
	r       *Reader
	beamSet *mesh.BeamSet
}

func (d *beamRefDecoder) Open() error                                        { return nil }
func (d *beamRefDecoder) Close() error                                       { return nil }
func (d *beamRefDecoder) Child(name xml.Name) (child nodeDecoder) { return }

func (d *beamRefDecoder) Attributes(attrs []xml.Attr) (err error) {
	var index uint64
	for _, a := range attrs {
		if a.Name.Space == "" && a.Name.Local == attrIndex {
			index, err = strconv.ParseUint(a.Value, 10, 32)
			break
		}
	}
	if err == nil {
		d.beamSet.Refs = append(d.beamSet.Refs, uint32(index))
	}
	return
}
