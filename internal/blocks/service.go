package blocks

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repo              Repo
	rebalanceWorker   *RebalanceWorker
	lexoRankThreshold int
}

func NewService(
	repo Repo,
	rebalanceWorker *RebalanceWorker,
	lexoRankThreshold int,
) *Service {
	return &Service{
		repo:              repo,
		rebalanceWorker:   rebalanceWorker,
		lexoRankThreshold: lexoRankThreshold,
	}
}

var (
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrSnapshotNotDraft = errors.New(
		"cannot modify blocks in a non-draft snapshot",
	)
	ErrInvalidBlockForMoveAfter = errors.New(
		"block cannot be moved after itself",
	)
	ErrBlockNotFound      = errors.New("block not found")
	ErrAfterBlockNotFound = errors.New(
		"after_block_id does not exist in this snapshot",
	)
	ErrPermissionDenied  = errors.New("permission denied")
	ErrTaskGroupNotFound = errors.New("task group not found")
	ErrTaskNameTaken     = errors.New(
		"a task with this name already exists in the task group",
	)
	ErrTaskGroupNameTaken = errors.New(
		"a task group with this name already exists in the course",
	)
	ErrInvalidTaskGroup = errors.New(
		"task group must have either an id or a new name of 1 to 128 characters",
	)
	ErrInvalidBlockType = errors.New(
		"block type must be 1 to 64 characters of a-z, 0-9 and -",
	)
	ErrInvalidBlockData = errors.New(
		"data must be a JSON object with an integer version, at most 64 KiB, and is not allowed for task blocks",
	)
	ErrBlockTypeChanged = errors.New("block type cannot be changed")
)

const (
	taskBlockType    = "task"
	maxBlockDataSize = 64 << 10
	maxTaskGroupName = 128
)

var blockTypeRegexp = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

func validateTaskGroupRef(ref TaskGroupRef) error {
	if (ref.ID == nil) == (ref.New == nil) {
		return ErrInvalidTaskGroup
	}
	if ref.New != nil &&
		(ref.New.Name == "" || len([]rune(ref.New.Name)) > maxTaskGroupName) {
		return ErrInvalidTaskGroup
	}
	return nil
}

// validateBlockData checks the only things the backend knows about a block's
// content: it is a JSON object carrying an integer version and is not too
// large. The rest of it belongs to the frontend.
func validateBlockData(data json.RawMessage) error {
	if len(data) > maxBlockDataSize {
		return ErrInvalidBlockData
	}
	var content map[string]json.RawMessage
	if err := json.Unmarshal(data, &content); err != nil || content == nil {
		return ErrInvalidBlockData
	}
	var version int64
	if err := json.Unmarshal(content["version"], &version); err != nil {
		return ErrInvalidBlockData
	}
	return nil
}

// CreateBlock creates a block (and, if it's a task-type block, its task data
// alongside it), enqueueing a lexorank rebalance if positions have grown too long.
func (s *Service) CreateBlock(
	ctx context.Context,
	command CreateBlockCommand,
) (*CreatedBlock, error) {
	if !blockTypeRegexp.MatchString(command.BlockType) {
		return nil, ErrInvalidBlockType
	}
	if command.BlockType == taskBlockType {
		if len(command.Data) > 0 || command.Task == nil {
			return nil, ErrInvalidBlockData
		}
		if err := validateTaskGroupRef(command.Task.TaskGroup); err != nil {
			return nil, err
		}
	} else if err := validateBlockData(command.Data); err != nil {
		return nil, err
	}

	result, err := s.repo.CreateBlock(ctx, command)
	if err != nil {
		return nil, err
	}

	if result.PositionLength > s.lexoRankThreshold {
		s.rebalanceWorker.Enqueue(result.SnapshotID)
	}

	return result, nil
}

// PatchBlock updates a block's own data. A task block has none.
func (s *Service) PatchBlock(
	ctx context.Context,
	command PatchBlockCommand,
) (*PatchedBlock, error) {
	if len(command.Data) > 0 {
		if err := validateBlockData(command.Data); err != nil {
			return nil, err
		}
	}
	return s.repo.PatchBlock(ctx, command)
}

// MoveBlock changes block's position so that it comes after afterBlockID
func (s *Service) MoveBlock(
	ctx context.Context,
	editCtx EditContext,
	ref BlockRef,
	afterBlockID *uuid.UUID,
) error {
	if afterBlockID != nil && *afterBlockID == ref.BlockID {
		return ErrInvalidBlockForMoveAfter
	}

	newPosition, err := s.repo.MoveBlock(ctx, editCtx, ref, afterBlockID)
	if err != nil {
		return err
	}

	// Check if rebalance is needed
	if len(newPosition) > s.lexoRankThreshold {
		s.rebalanceWorker.Enqueue(ref.SnapshotID)
	}

	return nil
}

func (s *Service) DeleteBlockByID(
	ctx context.Context,
	editCtx EditContext,
	ref BlockRef,
) error {
	return s.repo.DeleteBlockByID(ctx, editCtx, ref)
}

func (s *Service) GetBlockByID(
	ctx context.Context,
	editCtx EditContext,
	ref BlockRef,
) (*Block, error) {
	return s.repo.GetBlockByID(ctx, editCtx, ref)
}

func (s *Service) GetAllBlocksBySnapshotID(
	ctx context.Context,
	snapshotID uuid.UUID,
) ([]*Block, error) {
	return s.repo.GetAllBlocksBySnapshotID(ctx, snapshotID)
}

// Fractional indexing
const (
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	minChar  = '0'
	midChar  = 'V'
)

// CalculateMiddlePosition calculates a string that falls lexicographically
// between two other strings
func CalculateMiddlePosition(prev, next string) string {
	// Case 1: both values are empty (blocks table is empty)
	if prev == "" && next == "" {
		return string(midChar)
	}

	// Case 2: insertion at the top
	if prev == "" {
		runes := []rune(next)
		for i := len(runes) - 1; i >= 0; i-- {
			idx := strings.IndexRune(alphabet, runes[i])
			if idx > 0 {
				// If we decrease the character and it becomes equal to minChar (0),
				// we add midChar to give a buffer for future moves (0V).
				if idx-1 == 0 {
					return string(
						runes[:i],
					) + string(
						alphabet[0],
					) + string(
						midChar,
					)
				}
				runes[i] = rune(alphabet[idx-1])
				return string(runes[:i+1])
			}
		}
		return string(minChar) + string(midChar)
	}

	// Case 3: insertion at the bottom
	if next == "" {
		runes := []rune(prev)
		for i := len(runes) - 1; i >= 0; i-- {
			idx := strings.IndexRune(alphabet, runes[i])
			if idx < len(alphabet)-1 {
				runes[i] = rune(alphabet[idx+1])
				return string(runes[:i+1])
			}
		}
		return prev + string(midChar)
	}

	// Case 4: insertion in the middle
	var result strings.Builder

	for i := 0; ; i++ {
		pIdx := 0
		if i < len(prev) {
			pIdx = strings.IndexByte(alphabet, prev[i])
		}

		nIdx := len(alphabet) - 1
		if i < len(next) {
			nIdx = strings.IndexByte(alphabet, next[i])
		}

		// If the last chars of positions are the same, increment the length
		if pIdx == nIdx {
			result.WriteByte(alphabet[pIdx])
			continue
		}

		// If there is a free space between positions, calculate the middle
		if nIdx-pIdx > 1 {
			midIdx := pIdx + (nIdx-pIdx)/2
			result.WriteByte(alphabet[midIdx])
			break
		}

		// If the difference is 1, increment the length
		result.WriteByte(alphabet[pIdx])

		next = ""
	}

	return result.String()
}

// IndexToPosition is used by the rebalance worker to convert
// the integer index to a 4-char position in a 62-char alphabet
func IndexToPosition(num, denom int) string {
	var result []byte
	base := len(alphabet)

	for range 4 {
		num *= base
		idx := num / denom
		if idx >= base { // defensive, num < denom*base should make this unreachable
			idx = base - 1
		}

		result = append(result, alphabet[idx])

		num %= denom
		if num == 0 {
			break
		}
	}
	return string(result)
}
