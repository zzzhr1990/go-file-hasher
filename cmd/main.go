package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zzzhr1990/go-file-hasher/bthash"
)

func explorer(path string, info os.FileInfo, err error) error {
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return nil
	}
	if !info.IsDir() {
		hasher, err := bthash.CreateNewHasher(path, -1, context.Background())
		if err == nil {
			fmt.Printf("found: %s -> %s\n", path, hasher.RootString())
			err = validateFile(hasher, path)
			if err != nil {
				fmt.Printf("validate fail: %s -> %s\n", path, hasher.RootString())
				panic(err)
			}
		}

	}
	return nil
}
func validateFile(hasher *bthash.FileHasher, path string) error {
	var pieceLength int64 = 65536
	// blocksPerPiece := pieceLength / bthash.BlockSize // BLOCK_SIZE
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err != nil {
		return err
	}

	// pieces := make([][]byte, 0)
	i := 0
	for {
		piece := make([]byte, pieceLength)
		blockRead, err := f.Read(piece)
		if err != nil && err != io.EOF {
			return err
		}
		if blockRead == 0 {
			break
		}
		data := piece[:blockRead]
		//pieces = append(pieces, )
		pcs := bthash.CalcPieces(data, pieceLength, i == 0)
		if !bytes.Equal(pcs, hasher.Piecesv2[i]) {
			return errors.New("not match pcs")
		} else {
			println("piece validate ok!")
		}
		i++
	}
	return nil
}
func main() {
	println("Hello, world.")

	// bthash.CreateNewHasher("/Volumes/Code/.fseventsd/000000000148fdcd", -1, context.Background())

	filepath.Walk("/Volumes/Code", explorer)
}
