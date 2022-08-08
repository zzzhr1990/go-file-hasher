package bthash

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"hash/crc32"
	"io"
	"math/bits"
	"os"
	"strconv"
)

const (
	BlockSize = 16384
)

type FileHasher struct {
	Path      string
	Length    int64
	Piecesv2  [][]byte
	Piecesv1  [][]byte
	Sha1      []byte
	HeadSha1  []byte
	padLength int64
	padHasher hash.Hash
	Root      []byte
}

func CreateNewHasher(path string, pieceLength int64) (*FileHasher, error) {
	if pieceLength < 1 {
		pieceLength = 65536
	}
	hasher := &FileHasher{
		Path:     path,
		Length:   0,
		Piecesv1: make([][]byte, 0),
		Piecesv2: make([][]byte, 0),
	}

	//self.piecesv1 = []
	//    self.piecesv2 = []
	blocksPerPiece := pieceLength / BlockSize // BLOCK_SIZE
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err != nil {
		return nil, err
	}
	sha1Hasher := sha1.New()
	headSha1Hasher := sha1.New()
	byteHeadReadLeft := 128 * 1024
	for {
		residue := pieceLength
		blocks := [][]byte{}
		v1hasher := sha1.New()
		var i int64
		for i = 0; i < blocksPerPiece; i++ {
			block := make([]byte, BlockSize)
			blockRead, err := f.Read(block)
			if err != nil && err != io.EOF {
				return nil, err
			}
			if blockRead == 0 {
				break
			}
			actRead := block[:blockRead]
			hasher.Length += int64(blockRead)
			residue -= int64(blockRead)
			s256 := sha256.New()
			s256.Write(actRead)
			sx := s256.Sum(nil)
			blocks = append(blocks, sx)
			v1hasher.Write(actRead)
			sha1Hasher.Write(actRead)
			if byteHeadReadLeft > 0 {
				needRead := blockRead - byteHeadReadLeft
				if needRead > 0 {
					r := actRead[:needRead]
					byteHeadReadLeft -= needRead
					headSha1Hasher.Write(r)
				} else {
					byteHeadReadLeft -= blockRead
					headSha1Hasher.Write(actRead)
				}
			}
		}
		if len(blocks) == 0 {
			break
		}
		if len(blocks) != int(blocksPerPiece) {
			var leavesRequired int64
			if len(hasher.Piecesv2) == 0 { // first block
				leavesRequired = 1 << bits.Len(uint(len(blocks))-1) // find the smallest power of 2 that is >= len(blocks)
			} else {
				leavesRequired = blocksPerPiece
			}
			for i := 0; i < int(leavesRequired-int64(len(blocks))); i++ {
				blocks = append(blocks, make([]byte, 32))
			}
			// blocks for the last piece are padded with 0s
			// blocks = blocks + [b'\x00' * 32] * (leaves_required - len(blocks))
			// rootHash requires the number of leaves to be a power of 2
		}
		hasher.Piecesv2 = append(hasher.Piecesv2, rootHash(blocks)) //append(root_hash(blocks))
		if residue > 0 {
			hasher.padLength = residue
			hasher.padHasher = v1hasher
		} else {
			hasher.Piecesv1 = append(hasher.Piecesv1, v1hasher.Sum(nil))
		}

	}
	hasher.Sha1 = sha1Hasher.Sum(nil)
	hasher.HeadSha1 = headSha1Hasher.Sum(nil)
	if hasher.Length > 0 {
		layerHashes := hasher.Piecesv2
		if len(hasher.Piecesv2) > 1 {
			// # flatten piecesv2 into a single bytes object since that is what is needed for the 'piece layers' field
			// hasher.piecesv2 = bytes([byte for piece in self.piecesv2 for byte in piece])
			shh := sha256.New()
			for _, piece := range hasher.Piecesv2 {
				shh.Write(piece)
			}
			tHash := [][]byte{}
			for i := 0; i < int(blocksPerPiece); i++ {
				tHash = append(tHash, make([]byte, 32))
			}
			padPieceHash := rootHash(tHash)
			ix := (1 << bits.Len(uint(len(layerHashes)-1))) - len(layerHashes)
			// leavesRequired = int64(bits.Len(uint(1 << (len(blocks) - 1))))
			for i := 0; i < ix; i++ {
				layerHashes = append(layerHashes, padPieceHash)
			}
			// layer_hashes.extend([pad_piece_hash for i in range()])
		}
		hasher.Root = rootHash(layerHashes)
	}

	//fc := (fSize + pieceLength - 1) / pieceLength
	//if int64(len(hasher.Piecesv2)) != fc {
	//	panic("hasher.Piecesv2 != fc, path:" + path)
	//}

	return hasher, nil
}

func (hasher *FileHasher) AppendPadding() []byte {
	hasher.padHasher.Write(make([]byte, hasher.padLength))
	return hasher.padHasher.Sum(nil)
}

func (hasher *FileHasher) DiscardPadding() []byte {
	return hasher.padHasher.Sum(nil)
}

func (hasher *FileHasher) Sha1String() string {
	return hex.EncodeToString(hasher.Sha1)
}

func (hasher *FileHasher) HeadSha1String() string {
	return hex.EncodeToString(hasher.HeadSha1)
}

func (hasher *FileHasher) RootString() string {
	return hex.EncodeToString(hasher.Root)
}

func (hasher *FileHasher) UniqueID() string {

	prefix := strconv.FormatInt(hasher.Length, 36)
	if hasher.Length > 0 {
		prefix = prefix + "_" + hex.EncodeToString(hasher.Root)
	}
	return prefix + "_" + strconv.FormatInt(int64(crc32.ChecksumIEEE([]byte(prefix))), 36)
}

func rootHash(hashes [][]byte) []byte {

	for len(hashes) > 1 {
		if len(hashes)%2 != 0 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}
		newHashes := [][]byte{}
		for i := 0; i < len(hashes); i += 2 {
			s2 := sha256.New()
			s2.Write(append(hashes[i], hashes[i+1]...))
			newHashes = append(newHashes, s2.Sum(nil))
		}
		hashes = newHashes
	}
	return hashes[0]
}
