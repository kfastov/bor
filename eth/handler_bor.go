package eth

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/bor"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	// errCheckpointCount is returned when we are unable to fetch
	// the checkpoint count from local heimdall.
	errCheckpointCount = errors.New("failed to fetch checkpoint count")

	// errCheckpoint is returned when we are unable to fetch the
	// latest checkpoint from the local heimdall.
	errCheckpoint = errors.New("failed to fetch latest checkpoint")

	// errMissingCheckpoint is returned when we don't have the
	// checkpoint blocks locally, yet.
	errMissingCheckpoint = errors.New("missing checkpoint blocks")

	// errRootHash is returned when we aren't able to calculate the root hash
	// locally for a range of blocks.
	errRootHash = errors.New("failed to get local root hash")

	// errCheckpointRootHashMismatch is returned when the local root hash
	// doesn't match with the root hash in checkpoint.
	errCheckpointRootHashMismatch = errors.New("checkpoint roothash mismatch")

	// errEndBlock is returned when we're unable to fetch a block locally.
	errEndBlock = errors.New("failed to get end block")
)

// fetchWhitelistCheckpoints fetches the latest checkpoint/s from it's local heimdall
// and verifies the data against bor data.
func (h *ethHandler) fetchWhitelistCheckpoints(bor *bor.Bor, first bool) ([]uint64, []common.Hash, error) {
	// Create an array for block number and block hashes
	var (
		blockNums   []uint64
		blockHashes []common.Hash
	)

	// Fetch the checkpoint count from heimdall
	count, err := bor.HeimdallClient.FetchCheckpointCount()
	if err != nil || count == 0 {
		log.Debug("Failed to fetch checkpoint count for whitelisting", "err", err)
		return blockNums, blockHashes, errCheckpointCount
	}

	// If we're in the first iteration, we'll fetch last 10 checkpoints, else only the latest one
	iterations := 1
	if first {
		iterations = 10
	}

	for i := 0; i < iterations; i++ {
		// If we don't have any checkpoints in heimdall, break
		if count == 0 {
			break
		}

		// fetch `count` indexed checkpoint from heimdall
		checkpoint, err := bor.HeimdallClient.FetchCheckpoint(int64(count))
		if err != nil {
			log.Debug("Failed to fetch latest checkpoint for whitelisting", "err", err)
			return blockNums, blockHashes, errCheckpoint
		}

		// check if we have the checkpoint blocks
		head := h.ethAPI.BlockNumber()
		if head < hexutil.Uint64(checkpoint.EndBlock.Uint64()) {
			log.Debug("Head block behind checkpoint block", "head", head, "checkpoint end block", checkpoint.EndBlock)
			return blockNums, blockHashes, errMissingCheckpoint
		}

		// verify the root hash of checkpoint
		roothash, err := h.ethAPI.GetRootHash(context.Background(), checkpoint.StartBlock.Uint64(), checkpoint.EndBlock.Uint64())
		if err != nil {
			log.Debug("Failed to get root hash of checkpoint while whitelisting", "err", err)
			return blockNums, blockHashes, errRootHash
		}

		if roothash != checkpoint.RootHash.String()[2:] {
			log.Warn("Checkpoint root hash mismatch while whitelisting", "expected", checkpoint.RootHash.String()[2:], "got", roothash)
			return blockNums, blockHashes, errCheckpointRootHashMismatch
		}

		// fetch the end checkpoint block hash
		block, err := h.ethAPI.GetBlockByNumber(context.Background(), rpc.BlockNumber(checkpoint.EndBlock.Uint64()), false)
		if err != nil {
			log.Debug("Failed to get end block hash of checkpoint while whitelisting", "err", err)
			return blockNums, blockHashes, errEndBlock
		}
		hash := fmt.Sprintf("%v", block["hash"])
		blockNums = append(blockNums, checkpoint.EndBlock.Uint64())
		blockHashes = append(blockHashes, common.HexToHash(hash))
		count--
	}

	return blockNums, blockHashes, nil
}