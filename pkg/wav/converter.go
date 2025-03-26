package wav

import (
	"bytes"
	"encoding/binary"
)

func ConvertPCMToWAV(pcmData []byte, channels int, sampleRate int) ([]byte, error) {
	var buffer bytes.Buffer

	// Write WAV header
	binary.Write(&buffer, binary.LittleEndian, []byte("RIFF"))
	binary.Write(&buffer, binary.LittleEndian, uint32(len(pcmData)+36))
	binary.Write(&buffer, binary.LittleEndian, []byte("WAVE"))

	// "fmt " chunk
	binary.Write(&buffer, binary.LittleEndian, []byte("fmt "))
	binary.Write(&buffer, binary.LittleEndian, uint32(16))
	binary.Write(&buffer, binary.LittleEndian, uint16(1))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate*channels*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(16))

	// "data" chunk
	binary.Write(&buffer, binary.LittleEndian, []byte("data"))
	binary.Write(&buffer, binary.LittleEndian, uint32(len(pcmData)))
	binary.Write(&buffer, binary.LittleEndian, pcmData)

	return buffer.Bytes(), nil
}
