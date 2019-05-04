package decouplet

const errorMatchNotFound string = "match not found"
const partialStart string = ";[&"
const partialEnd string = "&];"

var partialStartBytes = []byte(partialStart)
var partialEndBytes = []byte(partialEnd)

type TranscoderType string

type DictionaryChars string

type ByteGroup struct {
	bytes []byte
}

type DecodeGroup struct {
	kind  []uint8
	place []string
}

type Dictionary struct {
	decoders []Decoder
}

type Decoder struct {
	character uint8
	amount    uint8
}

type location struct {
	x int
	y int
}

func (chars DictionaryChars) CheckIn(a byte) bool {
	for i := range chars {
		if a == chars[i] {
			return true
		}
	}
	return false
}
