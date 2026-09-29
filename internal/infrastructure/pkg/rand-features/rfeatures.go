package randfeatures

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// k - quantity of winners

func GenerateRandOffset(k int, actualNumberOfReferrers int) (offset []int, err error) {
	if k > actualNumberOfReferrers {
		k = actualNumberOfReferrers
	}

	seen := make(map[int]struct{}, k)
	offsets := make([]int, 0, k)

	max := big.NewInt(int64(actualNumberOfReferrers))

	for len(offsets) < k {

		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random offset: %w", err)
		}

		val := int(n.Int64())

		if _, exists := seen[val]; !exists {
			seen[val] = struct{}{}
			offsets = append(offsets, val)
		}
	}
	return offsets, nil

}
