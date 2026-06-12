package types

import (
	"strings"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// TaskContract is a small helper wrapper around the protobuf IOTask contract.
type TaskContract struct {
	Task *pb.IOTask
}

func NewTaskContract(task *pb.IOTask) TaskContract {
	return TaskContract{Task: task}
}

func (t TaskContract) HasTensorPath() bool {
	return t.TensorPreprocessID() != ""
}

func (t TaskContract) TensorPreprocessID() string {
	if t.Task == nil {
		return ""
	}
	return strings.TrimSpace(t.Task.GetTensorPreprocessId())
}

func (t TaskContract) TensorBatchingSupported() bool {
	if t.Task == nil {
		return false
	}
	return t.Task.GetTensorBatchingSupported()
}

func IOTaskHasTensorPath(task *pb.IOTask) bool {
	return NewTaskContract(task).HasTensorPath()
}

func IOTaskTensorPreprocessID(task *pb.IOTask) string {
	return NewTaskContract(task).TensorPreprocessID()
}

func IOTaskTensorBatchingSupported(task *pb.IOTask) bool {
	return NewTaskContract(task).TensorBatchingSupported()
}
