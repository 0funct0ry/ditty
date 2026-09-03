package wire

import "testing"

func FuzzDecode(f *testing.F) {
	seeds := [][]byte{
		{},
		{0xFE},
		append([]byte{byte(OpInput)}, []byte("hello")...),
		append([]byte{byte(OpOutput)}, []byte("\x1b[2Jworld")...),
		append([]byte{byte(OpResize)}, []byte(`{"cols":80,"rows":24}`)...),
		append([]byte{byte(OpResize)}, []byte(`{"cols":0,"rows":0}`)...),
		append([]byte{byte(OpResize)}, []byte(`{"cols":99999}`)...),
		{byte(OpPing)},
		{byte(OpPong)},
		append([]byte{byte(OpHello)}, []byte(`{"protocol":"ditty.v1"}`)...),
		append([]byte{byte(OpRoster)}, []byte(`{"count":0}`)...),
		append([]byte{byte(OpState)}, []byte(`{"state":"live"}`)...),
		append([]byte{byte(OpExit)}, []byte(`{"code":0}`)...),
		append([]byte{byte(OpNotice)}, []byte(`{"level":"info","message":"x"}`)...),
		append([]byte{byte(OpResize)}, []byte(`{"cols":`)...),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, frame []byte) {
		op, payload, err := Decode(frame)
		if err != nil {
			return
		}
		// Byte values 0x01/0x02 are shared between the two opcode tables
		// (Resize/Hello, Ping/Roster); try every typed decode that could
		// apply to this byte value rather than picking one direction.
		switch byte(op) {
		case 0x01:
			_, _ = DecodeResize(payload)
			_, _ = DecodeHello(payload)
		case 0x02:
			_, _ = DecodeRoster(payload)
		case byte(OpState):
			_, _ = DecodeState(payload)
		case byte(OpExit):
			_, _ = DecodeExit(payload)
		case byte(OpNotice):
			_, _ = DecodeNotice(payload)
		}
	})
}
