package network

type Request interface {
	GetConnection() Connection

	GetMessageData() []byte

	GetMessagePackageType() uint32
}

type request struct {
	conn Connection

	msg Message
}

func (r *request) GetConnection() Connection {
	return r.conn
}

func (r *request) GetMessageData() []byte {
	return r.msg.GetData()
}

func (r *request) GetMessagePackageType() uint32 {
	return r.msg.GetPackageType()
}
