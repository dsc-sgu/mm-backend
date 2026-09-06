package content

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

type repoStub struct {
	called bool
}

func (r *repoStub) CreateBlockContent(_ context.Context, command CreateBlockCommand) (*CreatedBlockContent, error) {
	r.called = command.BlockType == "task" && command.Task != nil
	return &CreatedBlockContent{BlockID: uuid.New()}, nil
}

func (r *repoStub) PatchBlockContent(_ context.Context, _ PatchBlockCommand) (*PatchedBlockContent, error) {
	return &PatchedBlockContent{}, nil
}

type rebalanceNotifierStub struct{}

func (rebalanceNotifierStub) Enqueue(uuid.UUID) {}

func TestServiceCreateBlockDelegatesToRepo(t *testing.T) {
	repo := &repoStub{}
	service := NewService(repo, rebalanceNotifierStub{}, 20)

	_, err := service.CreateBlock(context.Background(), CreateBlockCommand{
		BlockType: "task",
		Data:      json.RawMessage(`{"title":"test"}`),
		Task:      &TaskData{},
	})
	if err != nil {
		t.Fatalf("CreateBlock returned error: %v", err)
	}
	if !repo.called {
		t.Fatal("CreateBlock did not pass the task subtype to the repository")
	}
}
