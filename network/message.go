package network

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/homey/config"
)

type Message interface {
	// get package type
	GetPackageType() uint32

	// get message data
	GetData() []byte

	// get message data length
	GetDataLength() uint32

	// set package type
	SetPackageType(messageType uint32)

	// set message data
	SetData(data []byte)

	// set message data length
	SetDataLength(dataLength uint32)
}

type message struct {

	// message type for binding router
	PackageType uint32

	// message data length
	DataLength uint32

	// message data
	Data []byte
}

func (m *message) GetPackageType() uint32 {
	return m.PackageType
}

func (m *message) GetData() []byte {
	return m.Data
}

func (m *message) GetDataLength() uint32 {
	return m.DataLength
}

func (m *message) SetPackageType(Type uint32) {
	m.PackageType = Type
}

func (m *message) SetData(data []byte) {
	m.Data = data
}

func (m *message) SetDataLength(dataLength uint32) {
	m.DataLength = dataLength
}

func NewMessage(packageType uint32, data []byte) Message {
	return &message{
		PackageType: packageType,
		DataLength:  uint32(len(data)),
		Data:        data,
	}
}

func GetHeadLength() int8 {
	return config.GlobalConfig.LengthByte + config.GlobalConfig.TypeByte
}

func Pack(message Message) (packageData []byte, err error) {
	dataBuff := bytes.NewBuffer([]byte{})
	if config.GlobalConfig.LengthByte > 0 {
		if err := binary.Write(dataBuff, binary.LittleEndian, message.GetDataLength()); err != nil {
			return packageData, err
		}
	}

	if config.GlobalConfig.TypeByte > 0 {
		if err := binary.Write(dataBuff, binary.LittleEndian, message.GetPackageType()); err != nil {
			return packageData, err
		}
	}

	if err := binary.Write(dataBuff, binary.LittleEndian, message.GetData()); err != nil {
		return packageData, err
	}

	packageData = dataBuff.Bytes()

	return packageData, err
}

func UnPack(binaryData []byte) (Message, error) {
	message := &message{}
	dataBuff := bytes.NewBuffer(binaryData)

	if config.GlobalConfig.LengthByte > 0 {
		if err := binary.Read(dataBuff, binary.LittleEndian, &message.DataLength); err != nil {
			return message, err
		}
	}

	if config.GlobalConfig.MaxPackageSize > 0 && message.DataLength > config.GlobalConfig.MaxPackageSize {
		return message, fmt.Errorf("data length [%d] beyond max package size limit", message.DataLength)
	}

	if config.GlobalConfig.TypeByte > 0 {
		if err := binary.Read(dataBuff, binary.LittleEndian, &message.PackageType); err != nil {
			return message, err
		}
	}

	return message, nil
}
