package decouplet

import (
	"io"
	"sync"
)

type encodingKey interface {
	GetType() encoderType
	GetDictionaryChars() dictionaryChars
	GetDictionary() dictionary
}

func encode(
	input []byte,
	key encodingKey,
	encoder func(byte, encodingKey) ([]byte, error),
) (output []byte, err error) {
	bytes, err := WriteVersion(key.GetType())
	if err != nil {
		return nil, err
	}

	byteGroups := make([]byteGroup, len(input))
	errorCh := make(chan error, len(input))
	wg := &sync.WaitGroup{}
	wg.Add(len(input))

	for i := range input {
		bytesRelay(
			i, input, byteGroups, key, encoder, errorCh, wg)
	}

	select {
	case err := <-errorCh:
		return nil, err
	default:
		break
	}

	for _, byteGroup := range byteGroups {
		for _, b := range byteGroup.bytes {
			bytes = append(bytes, b)
		}
	}
	return bytes, nil
}

func encodeStream(
	input io.Reader,
	key encodingKey,
	encoder func(byte, encodingKey) ([]byte, error),
) (reader io.Reader, err error) {
	reader, writer := io.Pipe()
	go func(
		input io.Reader,
		writer *io.PipeWriter,
		encoder func(byte, encodingKey) ([]byte, error),
		key encodingKey) {
		for {
			b := make([]byte, 1)
			_, err := input.Read(b)
			if err != nil {
				if err == io.EOF {
					writer.Close()
					return
				}
				writer.CloseWithError(err)
				return
			}
			m, err := encoder(b[0], key)
			if err != nil {
				writer.CloseWithError(err)
			}
			_, err = writer.Write(m)
			if err != nil {
				writer.CloseWithError(err)
			}
		}
	}(input, writer, encoder, key)
	return reader, nil
}

func encodePartialStream(
	input io.Reader,
	key encodingKey,
	take int,
	skip int,
	encoder func(byte, encodingKey) ([]byte, error),
) (reader io.Reader, err error) {
	reader, writer := io.Pipe()

	go writeEncodeStreamPartial(
		input, writer, key, take, skip, encoder)

	return reader, nil
}

func writeEncodeStreamPartial(
	input io.Reader,
	writer *io.PipeWriter,
	key encodingKey,
	take int,
	skip int,
	encoder func(byte, encodingKey) ([]byte, error),
) {
	defer writer.Close()
	for {
		_, err := writer.Write(partialStartBytes)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		takeR := io.LimitReader(input, int64(take))
		EncodedR, err := encodeStream(takeR, key, encoder)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		_, err = io.Copy(writer, EncodedR)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		_, err = writer.Write(partialEndBytes)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		_, err = io.CopyN(writer, input, int64(skip))
		if err != nil {
			writer.CloseWithError(err)
			return
		}
	}
}

func encodeConcurrent(
	input []byte,
	key encodingKey,
	encoder func(byte, encodingKey) ([]byte, error),
) (output []byte, err error) {
	bytes, err := WriteVersion(key.GetType())
	if err != nil {
		return nil, err
	}

	byteGroups := make([]byteGroup, len(input))
	errorCh := make(chan error, len(input))
	wg := &sync.WaitGroup{}
	wg.Add(len(input))

	for i := range input {
		go bytesRelay(
			i, input, byteGroups, key, encoder, errorCh, wg)
	}
	wg.Wait()

	select {
	case err := <-errorCh:
		return nil, err
	default:
		break
	}

	for _, byteGroup := range byteGroups {
		for _, b := range byteGroup.bytes {
			bytes = append(bytes, b)
		}
	}
	return bytes, nil
}

func bytesRelay(
	index int,
	input []byte,
	bytes []byteGroup,
	key encodingKey,
	encoder func(byte, encodingKey) ([]byte, error),
	errorCh chan error,
	wg *sync.WaitGroup) {
	defer wg.Done()
	byteGroup := byteGroup{
		bytes: make([]byte, 0),
	}
	msg, err := encoder(input[index], key)
	if err != nil {
		errorCh <- err
		return
	}
	for _, b := range msg {
		byteGroup.bytes = append(byteGroup.bytes, b)
	}
	bytes[index] = byteGroup
}
