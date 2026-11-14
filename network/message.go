package network

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/towerman1990/homey/config"
)

type messageCodec struct {
	order    binary.ByteOrder
	settings config.GlobalConfig
}

func newMessageCodec(cfg config.GlobalConfig) messageCodec {
	var order binary.ByteOrder = binary.BigEndian
	if cfg.Message.Endian == "little" {
		order = binary.LittleEndian
	}
	return messageCodec{order: order, settings: cfg}
}

type (
	Message interface {

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

	// message structure: connID->type->length->data
	message struct {

		// the ID of connection which is in charge of sending message
		// if connID != 0 indicate it's a forward message
		connID uint64

		// this field is used to bind router
		// when a request arrive, it would be handled by a router function according this value
		DataType uint32

		// message data length
		DataLength uint32

		// message data
		Data []byte
	}
)

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

func Pack(message Message) ([]byte, error) {
	return newMessageCodec(config.Global).pack(message)
}

func (c messageCodec) pack(message Message) (packageData []byte, err error) {
	if c.settings.TLV.Length && uint64(message.GetDataLength()) != uint64(len(message.GetData())) {
		return nil, fmt.Errorf("declared payload length does not match data")
	}
	dataBuff := bytes.NewBuffer([]byte{})

	if message.GetConnID() > 0 {
		if err := binary.Write(dataBuff, c.order, message.GetConnID()); err != nil {
			return packageData, err
		}
	}

	if c.settings.TLV.Type {
		if err := binary.Write(dataBuff, c.order, message.GetDataType()); err != nil {
			return packageData, err
		}
	}

	if c.settings.TLV.Length {
		if err := binary.Write(dataBuff, c.order, message.GetDataLength()); err != nil {
			return packageData, err
		}
	}

	if err := binary.Write(dataBuff, c.order, message.GetData()); err != nil {
		return packageData, err
	}

	packageData = dataBuff.Bytes()

	return packageData, err
}

func UnPack(binaryData []byte, isForward bool) (Message, error) {
	return newMessageCodec(config.Global).unpack(binaryData, isForward)
}

func (c messageCodec) unpack(binaryData []byte, isForward bool) (Message, error) {
	message := &message{}
	dataBuff := bytes.NewBuffer(binaryData)

	if isForward {
		if err := binary.Read(dataBuff, c.order, &message.connID); err != nil {
			return message, err
		}
	}

	if c.settings.TLV.Type {
		if err := binary.Read(dataBuff, c.order, &message.DataType); err != nil {
			return message, err
		}
	}

	if c.settings.TLV.Length {
		if err := binary.Read(dataBuff, c.order, &message.DataLength); err != nil {
			return message, err
		}
		if uint64(message.DataLength) != uint64(dataBuff.Len()) {
			return message, fmt.Errorf("declared payload length does not match data")
		}
	}

	if uint64(dataBuff.Len()) > uint64(^uint32(0)) ||
		(c.settings.MaxPackageSize > 0 && uint64(dataBuff.Len()) > uint64(c.settings.MaxPackageSize)) {
		return message, fmt.Errorf("message data length [%d] beyond max package size limit", dataBuff.Len())
	}
	message.DataLength = uint32(dataBuff.Len())
	message.Data = append([]byte(nil), dataBuff.Bytes()...)

	return message, nil
}
