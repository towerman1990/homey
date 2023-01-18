package network

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/homey/config"
)

var endian binary.ByteOrder

type Message interface {

	// get send message connection id
	GetConnID() uint64

	// get package type
	GetDataType() uint32

	// get message data
	GetData() []byte

	// get message data length
	GetDataLength() uint32

	// set eventually send message connection id
	SetConnID(connID uint64)

	// set package type
	SetDataType(messageType uint32)

	// set message data
	SetData(data []byte)

	// set message data length
	SetDataLength(dataLength uint32)
}

// message structure: connID->length->type->data
type message struct {

	// the ID of connection which is in charge of sending message
	// if connID != 0 indicate it's a forward message
	connID uint64

	// message type for binding router
	DataType uint32

	// message data length
	DataLength uint32

	// message data
	Data []byte
}

func init() {
	if config.GlobalConfig.Message.Endian == "little" {
		endian = binary.LittleEndian
	} else {
		endian = binary.BigEndian
	}
}

func (m *message) GetConnID() uint64 {
	return m.connID
}

func (m *message) GetDataType() uint32 {
	return m.DataType
}

func (m *message) GetData() []byte {
	return m.Data
}

func (m *message) GetDataLength() uint32 {
	return m.DataLength
}

func (m *message) SetConnID(connID uint64) {
	m.connID = connID
}

func (m *message) SetDataType(Type uint32) {
	m.DataType = Type
}

func (m *message) SetData(data []byte) {
	m.Data = data
}

func (m *message) SetDataLength(dataLength uint32) {
	m.DataLength = dataLength
}

func (m *message) GetHeadLength() int8 {
	if m.connID > 0 {
		return 4 + 8 + 4 // uint64 connID take 8 byte length
	}
	return 4 + 4
}

func NewMessage(packageType uint32, data []byte) Message {
	return &message{
		connID:     0,
		DataType:   packageType,
		DataLength: uint32(len(data)),
		Data:       data,
	}
}

func Pack(message Message) (packageData []byte, err error) {
	dataBuff := bytes.NewBuffer([]byte{})

	if message.GetConnID() > 0 {
		if err := binary.Write(dataBuff, endian, message.GetConnID()); err != nil {
			return packageData, err
		}
	}

	if config.GlobalConfig.TLV.Type {
		if err := binary.Write(dataBuff, endian, message.GetDataType()); err != nil {
			return packageData, err
		}
	}

	if config.GlobalConfig.TLV.Length {
		if err := binary.Write(dataBuff, endian, message.GetDataLength()); err != nil {
			return packageData, err
		}
	}

	if err := binary.Write(dataBuff, endian, message.GetData()); err != nil {
		return packageData, err
	}

	packageData = dataBuff.Bytes()

	return packageData, err
}

func UnPack(binaryData []byte, isForward bool) (Message, error) {
	message := &message{}
	dataBuff := bytes.NewBuffer(binaryData)

	if isForward {
		if err := binary.Read(dataBuff, endian, &message.connID); err != nil {
			return message, err
		}
	}

	if config.GlobalConfig.TLV.Type {
		if err := binary.Read(dataBuff, endian, &message.DataType); err != nil {
			return message, err
		}
	}

	if config.GlobalConfig.TLV.Length {
		if err := binary.Read(dataBuff, endian, &message.DataLength); err != nil {
			return message, err
		}
	} else {
		message.DataLength = uint32(len(binaryData))
		if message.connID != 0 {
			message.DataLength -= 8
		}
	}

	if config.GlobalConfig.MaxPackageSize > 0 && message.DataLength > config.GlobalConfig.MaxPackageSize {
		return message, fmt.Errorf("message data length [%d] beyond max package size limit", message.DataLength)
	}

	message.Data = make([]byte, message.DataLength)
	if err := binary.Read(dataBuff, endian, &message.Data); err != nil {
		return message, err
	}

	return message, nil
}
