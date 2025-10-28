package api

type IdMessage interface {
	GetId() string
}

func (m *WsMessagesChangedMessage) GetId() string {
	return m.Id
}

func (m *WsMessage) GetId() string {
	return m.Id
}
