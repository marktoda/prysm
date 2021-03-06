package hashutil

import (
	"errors"
	"hash"
	"reflect"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/minio/highwayhash"
	"github.com/minio/sha256-simd"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"golang.org/x/crypto/sha3"
)

// ErrNilProto can occur when attempting to hash a protobuf message that is nil
// or has nil objects within lists.
var ErrNilProto = errors.New("cannot hash a nil protobuf message")

var sha256Pool = sync.Pool{New: func() interface{} {
	return sha256.New()
}}

// Hash defines a function that returns the sha256 checksum of the data passed in.
// https://github.com/ethereum/eth2.0-specs/blob/v0.9.3/specs/core/0_beacon-chain.md#hash
func Hash(data []byte) [32]byte {
	h := sha256Pool.Get().(hash.Hash)
	defer sha256Pool.Put(h)
	h.Reset()

	var b [32]byte

	// The hash interface never returns an error, for that reason
	// we are not handling the error below. For reference, it is
	// stated here https://golang.org/pkg/hash/#Hash

	// #nosec G104
	h.Write(data)
	h.Sum(b[:0])

	return b
}

// CustomSHA256Hasher returns a hash function that uses
// an enclosed hasher. This is not safe for concurrent
// use as the same hasher is being called throughout.
//
// Note: that this method is only more performant over
// hashutil.Hash if the callback is used more than 5 times.
func CustomSHA256Hasher() func([]byte) [32]byte {
	hasher := sha256Pool.Get().(hash.Hash)
	hasher.Reset()
	var hash [32]byte

	return func(data []byte) [32]byte {
		// The hash interface never returns an error, for that reason
		// we are not handling the error below. For reference, it is
		// stated here https://golang.org/pkg/hash/#Hash

		// #nosec G104
		hasher.Write(data)
		hasher.Sum(hash[:0])
		hasher.Reset()

		return hash
	}
}

var keccak256Pool = sync.Pool{New: func() interface{} {
	return sha3.NewLegacyKeccak256()
}}

// HashKeccak256 defines a function which returns the Keccak-256/SHA3
// hash of the data passed in.
func HashKeccak256(data []byte) [32]byte {
	var b [32]byte

	h := keccak256Pool.Get().(hash.Hash)
	defer keccak256Pool.Put(h)
	h.Reset()

	// The hash interface never returns an error, for that reason
	// we are not handling the error below. For reference, it is
	// stated here https://golang.org/pkg/hash/#Hash

	// #nosec G104
	h.Write(data)
	h.Sum(b[:0])

	return b
}

// RepeatHash applies the sha256 hash function repeatedly
// numTimes on a [32]byte array.
func RepeatHash(data [32]byte, numTimes uint64) [32]byte {
	if numTimes == 0 {
		return data
	}
	return RepeatHash(Hash(data[:]), numTimes-1)
}

// HashProto hashes a protocol buffer message using sha256.
func HashProto(msg proto.Message) (result [32]byte, err error) {
	// Hashing a proto with nil pointers will cause a panic in the unsafe
	// proto.Marshal library.
	defer func() {
		if r := recover(); r != nil {
			err = ErrNilProto
		}
	}()

	if msg == nil || reflect.ValueOf(msg).IsNil() {
		return [32]byte{}, ErrNilProto
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return [32]byte{}, err
	}
	return Hash(data), nil
}

// Key used for FastSum64
var fastSumHashKey = bytesutil.ToBytes32([]byte("hash_fast_sum64_key"))

// FastSum64 returns a hash sum of the input data using highwayhash. This method is not secure, but
// may be used as a quick identifier for objects where collisions are acceptable.
func FastSum64(data []byte) uint64 {
	return highwayhash.Sum64(data, fastSumHashKey[:])
}

// FastSum256 returns a hash sum of the input data using highwayhash. This method is not secure, but
// may be used as a quick identifier for objects where collisions are acceptable.
func FastSum256(data []byte) [32]byte {
	return highwayhash.Sum(data, fastSumHashKey[:])
}
