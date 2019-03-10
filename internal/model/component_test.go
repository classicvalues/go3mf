package model

import (
	"reflect"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/qmuntal/go3mf/internal/mesh"
	"github.com/stretchr/testify/mock"
)

// MockMergeableMesh is a mock of MergeableMesh interface
type MockObject struct {
	mock.Mock
}

func NewMockObject(isValid, isValidForSlices bool) *MockObject {
	o := new(MockObject)
	o.On("IsValid").Return(isValid)
	o.On("IsValidForSlices", mock.Anything).Return(isValidForSlices)
	return o
}

func (o *MockObject) Type() ObjectType {
	return ObjectTypeOther
}

func (o *MockObject) MergeToMesh(args0 *mesh.Mesh, args1 mgl32.Mat4) error {
	o.Called(args0, args1)
	return nil
}

func (o *MockObject) IsValid() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *MockObject) IsValidForSlices(args0 mgl32.Mat4) bool {
	args := o.Called(args0)
	return args.Bool(0)
}

func TestComponent_HasTransform(t *testing.T) {
	tests := []struct {
		name string
		c    *Component
		want bool
	}{
		{"identity", &Component{Transform: mgl32.Ident4()}, false},
		{"base", &Component{Transform: mgl32.Mat4{2, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.HasTransform(); got != tt.want {
				t.Errorf("Component.HasTransform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponent_MergeToMesh(t *testing.T) {
	type args struct {
		m         *mesh.Mesh
		transform mgl32.Mat4
	}
	tests := []struct {
		name string
		c    *Component
		args args
	}{
		{"base", &Component{Object: new(ObjectResource)}, args{new(mesh.Mesh), mgl32.Ident4()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.MergeToMesh(tt.args.m, tt.args.transform)
		})
	}
}

func TestObjectResource_IsValid(t *testing.T) {
	tests := []struct {
		name string
		o    *ObjectResource
		want bool
	}{
		{"base", new(ObjectResource), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.IsValid(); got != tt.want {
				t.Errorf("ObjectResource.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponentResource_IsValid(t *testing.T) {
	tests := []struct {
		name string
		c    *ComponentResource
		want bool
	}{
		{"empty", new(ComponentResource), false},
		{"oneInvalid", &ComponentResource{Components: []*Component{{Object: NewMockObject(true, true)}, {Object: NewMockObject(false, true)}}}, false},
		{"valid", &ComponentResource{Components: []*Component{{Object: NewMockObject(true, true)}, {Object: NewMockObject(true, true)}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.IsValid(); got != tt.want {
				t.Errorf("ComponentResource.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectResource_IsValidForSlices(t *testing.T) {
	type args struct {
		transform mgl32.Mat4
	}
	tests := []struct {
		name string
		o    *ObjectResource
		args args
		want bool
	}{
		{"base", new(ObjectResource), args{mgl32.Ident4()}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.IsValidForSlices(tt.args.transform); got != tt.want {
				t.Errorf("ObjectResource.IsValidForSlices() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponentResource_IsValidForSlices(t *testing.T) {
	type args struct {
		transform mgl32.Mat4
	}
	tests := []struct {
		name string
		c    *ComponentResource
		args args
		want bool
	}{
		{"empty", new(ComponentResource), args{mgl32.Ident4()}, true},
		{"oneInvalid", &ComponentResource{Components: []*Component{{Object: NewMockObject(true, true)}, {Object: NewMockObject(true, false)}}}, args{mgl32.Ident4()}, false},
		{"valid", &ComponentResource{Components: []*Component{{Object: NewMockObject(true, true)}, {Object: NewMockObject(true, true)}}}, args{mgl32.Ident4()}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.IsValidForSlices(tt.args.transform); got != tt.want {
				t.Errorf("ComponentResource.IsValidForSlices() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponentResource_MergeToMesh(t *testing.T) {
	type args struct {
		m         *mesh.Mesh
		transform mgl32.Mat4
	}
	tests := []struct {
		name string
		c    *ComponentResource
		args args
	}{
		{"empty", new(ComponentResource), args{nil, mgl32.Ident4()}},
		{"base", &ComponentResource{Components: []*Component{{Object: new(ObjectResource)}}}, args{nil, mgl32.Ident4()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.MergeToMesh(tt.args.m, tt.args.transform)
		})
	}
}

func TestMeshResource_IsValidForSlices(t *testing.T) {
	type args struct {
		t mgl32.Mat4
	}
	tests := []struct {
		name string
		c    *MeshResource
		args args
		want bool
	}{
		{"empty", new(MeshResource), args{mgl32.Mat4{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}, true},
		{"valid", &MeshResource{ObjectResource: ObjectResource{SliceStackID: 0}}, args{mgl32.Mat4{1, 1, 0, 1, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1}}, true},
		{"invalid", &MeshResource{ObjectResource: ObjectResource{SliceStackID: 1}}, args{mgl32.Mat4{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.IsValidForSlices(tt.args.t); got != tt.want {
				t.Errorf("MeshResource.IsValidForSlices() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMeshResource_IsValid(t *testing.T) {
	tests := []struct {
		name string
		c    *MeshResource
		want bool
	}{
		{"empty", new(MeshResource), false},
		{"other", &MeshResource{Mesh: new(mesh.Mesh), ObjectResource: ObjectResource{ObjectType: ObjectTypeOther}}, false},
		{"surface", &MeshResource{Mesh: new(mesh.Mesh), ObjectResource: ObjectResource{ObjectType: ObjectTypeSurface}}, true},
		{"support", &MeshResource{Mesh: new(mesh.Mesh), ObjectResource: ObjectResource{ObjectType: ObjectTypeSupport}}, true},
		{"solidsupport", &MeshResource{Mesh: new(mesh.Mesh), ObjectResource: ObjectResource{ObjectType: ObjectTypeSolidSupport}}, false},
		{"model", &MeshResource{Mesh: new(mesh.Mesh), ObjectResource: ObjectResource{ObjectType: ObjectTypeModel}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.IsValid(); got != tt.want {
				t.Errorf("MeshResource.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMeshResource_MergeToMesh(t *testing.T) {
	type args struct {
		m         *mesh.Mesh
		transform mgl32.Mat4
	}
	tests := []struct {
		name    string
		c       *MeshResource
		args    args
		wantErr bool
	}{
		{"base", &MeshResource{Mesh: new(mesh.Mesh)}, args{new(mesh.Mesh), mgl32.Ident4()}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.MergeToMesh(tt.args.m, tt.args.transform); (err != nil) != tt.wantErr {
				t.Errorf("MeshResource.MergeToMesh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestObjectResource_Type(t *testing.T) {
	tests := []struct {
		name string
		o    *ObjectResource
		want ObjectType
	}{
		{"base", &ObjectResource{ObjectType: ObjectTypeModel}, ObjectTypeModel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.Type(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObjectResource.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}
