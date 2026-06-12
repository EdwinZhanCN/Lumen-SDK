package proto

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestThinTensorWireFields(t *testing.T) {
	var task IOTask
	assertWireFieldNumber(t, task.ProtoReflect().Descriptor().Fields(), "tensor_preprocess_id", 5)
	assertWireFieldNumber(t, task.ProtoReflect().Descriptor().Fields(), "tensor_batching_supported", 6)

	task.TensorPreprocessId = "siglip2_base_patch16_224_image_v1"
	task.TensorBatchingSupported = true
	if got := task.GetTensorPreprocessId(); got != "siglip2_base_patch16_224_image_v1" {
		t.Fatalf("GetTensorPreprocessId() = %q", got)
	}
	if !task.GetTensorBatchingSupported() {
		t.Fatalf("GetTensorBatchingSupported() = false, want true")
	}
}

func TestCapabilityProtocolVersionWireField(t *testing.T) {
	var capability Capability
	assertWireFieldNumber(t, capability.ProtoReflect().Descriptor().Fields(), "protocol_version", 8)

	capability.ProtocolVersion = "1.0"
	if got := capability.GetProtocolVersion(); got != "1.0" {
		t.Fatalf("GetProtocolVersion() = %q, want %q", got, "1.0")
	}
}

func assertWireFieldNumber(t *testing.T, fields protoreflect.FieldDescriptors, name protoreflect.Name, want protoreflect.FieldNumber) {
	t.Helper()
	field := fields.ByName(name)
	if field == nil {
		t.Fatalf("field %s is missing from generated protobuf binding", name)
	}
	if got := field.Number(); got != want {
		t.Fatalf("field %s number = %d, want %d", name, got, want)
	}
}
