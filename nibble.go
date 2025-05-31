package qp

// big-endian style, upper(0) lower(1)
func nibbleIndex(key1, key2 []byte) (index nibbleIndexT, match bool) {
	len1 := nibbleIndexT(len(key1))
	len2 := nibbleIndexT(len(key2))
	minLen := min(len1, len2)

	i, a, b := nibbleIndexT(0), uint8(0), uint8(0)
	for i < minLen {
		a = key1[i]
		b = key2[i]
		if a != b {
			if (a & 0xf0) == (b & 0xf0) {
				// a,b upper nibble equals, return lower nibble
				return 1 + i<<1, false
			} else {
				return i << 1, false
			}
		}
		i++
	}

	index = i << 1
	if len1 == len2 {
		return index, true
	}

	return index, false
}

func nibbleBit(index nibbleIndexT, key []byte) bitmapT {
	byteIndex := index >> 1
	if byteIndex >= nibbleIndexT(len(key)) {
		return bitmapT(1 << 0) // NO_BYTE
	}
	k := key[byteIndex]

	nibble := uint8(0)
	if index&1 == 1 {
		// lower nibble
		nibble = k & 0x0f
	} else {
		// upper nibble
		nibble = k >> 4
	}
	return 1 << (nibble + 1)
}
