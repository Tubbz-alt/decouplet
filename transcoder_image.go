package decouplet

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math/rand"
	"strconv"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

type imageKey struct {
	image.Image
}

func (imageKey) GetKeyType() encoderType {
	return encoderType("imgec")
}

func (imageKey) GetDictionaryChars() dictionaryChars {
	return dictionaryChars("rgbacmyk")
}

func (imageKey) GetDictionary() dictionary {
	return dictionary{
		decoders: []decodeRef{
			{
				character: 'r',
				amount:    0,
			},
			{
				character: 'g',
				amount:    0,
			},
			{
				character: 'b',
				amount:    0,
			},
			{
				character: 'a',
				amount:    0,
			},
			{
				character: 'c',
				amount:    0,
			},
			{
				character: 'm',
				amount:    0,
			},
			{
				character: 'y',
				amount:    0,
			},
			{
				character: 'k',
				amount:    0,
			},
		},
	}
}

func dictionaryRGBACMYK(col color.Color, dict dictionary) dictionary {
	r, g, b, a := col.RGBA()
	c, m, y, k := color.RGBToCMYK(uint8(r), uint8(g), uint8(b))
	for i := range dict.decoders {
		switch dict.decoders[i].character {
		case 'r':
			dict.decoders[i].amount = uint8(r)
		case 'g':
			dict.decoders[i].amount = uint8(g)
		case 'b':
			dict.decoders[i].amount = uint8(b)
		case 'a':
			dict.decoders[i].amount = uint8(a)
		case 'c':
			dict.decoders[i].amount = uint8(c)
		case 'm':
			dict.decoders[i].amount = uint8(m)
		case 'y':
			dict.decoders[i].amount = uint8(y)
		case 'k':
			dict.decoders[i].amount = uint8(k)
		}
	}
	return dict
}

func EncodeImage(input []byte, key image.Image) ([]byte, error) {
	return encode(
		input, imageKey{key}, findPixelPattern)
	return nil, nil
}

func EncodeImageStream(input io.Reader, key image.Image) (io.Reader, error) {
	return encodeStream(
		input, imageKey{key}, findPixelPattern)
}

func EncodeImageStreamPartial(input io.Reader, key image.Image, take int, skip int) (io.Reader, error) {
	return encodePartialStream(
		input, imageKey{key}, take, skip, findPixelPattern)
}

func EncodeImageConcurrent(input []byte, key image.Image) ([]byte, error) {
	return encodeConcurrent(
		input, imageKey{key}, findPixelPattern)
	return nil, nil
}

func DecodeImage(input []byte, key image.Image) ([]byte, error) {
	return decode(
		input, imageKey{key}, 2, getImgDefs)
}

func DecodeImageStream(input io.Reader, key image.Image) (io.Reader, error) {
	return decodeStream(
		input, imageKey{key}, 2, getImgDefs)
}

func DecodeImageStreamPartial(input io.Reader, key image.Image) (io.Reader, error) {
	return decodePartialStream(
		input, imageKey{key}, 2, getImgDefs)
}

func getImgDefs(key encodingKey, group decodeGroup) (byte, error) {
	if len(group.place) < 2 {
		return 0, errors.New("decode group missing locations")
	}
	img, ok := key.(imageKey)
	if !ok {
		return 0, errors.New("failed to cast encodingKey")
	}
	dict := key.GetDictionary()

	loc1, err := strconv.Atoi(group.place[0])
	if err != nil {
		return 0, err
	}
	loc2, err := strconv.Atoi(group.place[1])
	if err != nil {
		return 0, err
	}
	location1, err := getXYLocation(loc1, img.Bounds().Max.X)
	if err != nil {
		return 0, err
	}
	location2, err := getXYLocation(loc2, img.Bounds().Max.X)
	if err != nil {
		return 0, err
	}

	var change1 uint8
	var change2 uint8
	changeColor1 := img.At(location1.x, location1.y)
	changeColor2 := img.At(location2.x, location2.y)
	dict1 := dictionaryRGBACMYK(changeColor1, dict)
	dict2 := dictionaryRGBACMYK(changeColor2, dict)

	for _, g := range dict1.decoders {
		if g.character == group.kind[0] {
			change1 = g.amount
		}
	}
	for _, g := range dict2.decoders {
		if g.character == group.kind[1] {
			change2 = g.amount
		}
	}
	return change2 - change1, nil
}

func findPixelPattern(char byte, key encodingKey) ([]byte, error) {
	img, ok := key.(imageKey)
	if !ok {
		return nil, errors.New("failed to cast encodingKey")
	}
	bounds := img.Bounds()
	startX := rand.Intn(bounds.Max.X)
	startY := rand.Intn(bounds.Max.Y)
	firstColor := img.At(startX, startY)

	pattern, err := findPixelPartner(
		location{x: startX, y: startY}, char, firstColor, img, key.GetDictionary())
	if err != nil && err == errorMatchNotFound {
		startX = rand.Intn(bounds.Max.X)
		startY = rand.Intn(bounds.Max.Y)
		firstColor = img.At(startX, startY)

		pattern, err = findPixelPartner(
			location{x: startX, y: startY}, char, firstColor, img, key.GetDictionary())
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	return pattern, nil
}

func findPixelPartner(
	location location,
	difference byte,
	currentColor color.Color,
	img image.Image,
	dict dictionary) ([]byte, error) {
	bounds := img.Bounds()
	for x := 0; x < bounds.Max.X; x++ {
		for y := 0; y < bounds.Max.Y; y++ {
			checkedColor := img.At(x, y)
			if match, firstType, secondType := checkColorMatch(
				difference, currentColor, checkedColor, dict); match {
				firstLocation := getPixelNumber(
					location.x, location.y, bounds.Max.X)
				secondLocation := getPixelNumber(x, y, bounds.Max.X)
				return []byte(fmt.Sprintf(
					"%s%v%s%v",
					string(firstType), firstLocation,
					string(secondType), secondLocation)), nil
			}
		}
	}
	return nil, errorMatchNotFound
}

func checkColorMatch(
	diff byte,
	current color.Color,
	checked color.Color,
	dict dictionary) (bool, uint8, uint8) {
	currentColors := dictionaryRGBACMYK(current, dict)
	checkedColors := dictionaryRGBACMYK(checked, dict)
	for v := range currentColors.decoders {
		for k := range checkedColors.decoders {
			if checkedColors.decoders[k].amount ==
				currentColors.decoders[v].amount+uint8(diff) {
				return true,
					currentColors.decoders[v].character,
					currentColors.decoders[k].character
			}
		}
	}
	return false, 0, 0
}

func getXYLocation(loc int, imageWidth int) (location, error) {
	location := location{}
	x, y := getCoordinates(loc, imageWidth)
	location.x = x
	location.y = y
	return location, nil
}
