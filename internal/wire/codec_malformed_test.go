package wire

import "testing"

func TestDecodeMalformed(t *testing.T) {
	cases := []struct {
		name    string
		frame   []byte
		wantErr bool
	}{
		{"empty", []byte{}, true},
		{"unknown opcode", []byte{0xFE}, true},
		{"truncated JSON Resize", append([]byte{byte(OpResize)}, []byte(`{"cols":80,`)...), true},
		{"truncated JSON Hello", append([]byte{byte(OpHello)}, []byte(`{"protocol":`)...), true},
		{"empty JSON payload for Resize opcode", []byte{byte(OpResize)}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Decode(tc.frame)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Decode(%v) error = %v, wantErr %v", tc.frame, err, tc.wantErr)
			}
		})
	}
}

func TestDecodeResizeClampsOutOfRange(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantCols int
		wantRows int
	}{
		{"zero values clamp up", `{"cols":0,"rows":0}`, MinCols, MinRows},
		{"huge cols clamps down", `{"cols":99999,"rows":24}`, MaxCols, 24},
		{"huge rows clamps down", `{"cols":80,"rows":99999}`, 80, MaxRows},
		{"negative clamps up", `{"cols":-5,"rows":-5}`, MinCols, MinRows},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeResize([]byte(tc.in))
			if err != nil {
				t.Fatalf("DecodeResize: %v", err)
			}
			if got.Cols != tc.wantCols || got.Rows != tc.wantRows {
				t.Fatalf("got %+v, want cols=%d rows=%d", got, tc.wantCols, tc.wantRows)
			}
		})
	}
}

func TestDecodeResizeMalformed(t *testing.T) {
	if _, err := DecodeResize([]byte(`not json`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeWrongShape(t *testing.T) {
	// Valid JSON, but the Hello shape decoded as State: State.State stays
	// empty and must fail the enum check.
	helloJSON := []byte(`{"protocol":"ditty.v1","server":"1.0.0"}`)
	if _, err := DecodeState(helloJSON); err == nil {
		t.Fatal("expected error decoding Hello-shaped JSON as State")
	}
}

func TestDecodeNoticeInvalidLevel(t *testing.T) {
	if _, err := DecodeNotice([]byte(`{"level":"critical","message":"x"}`)); err == nil {
		t.Fatal("expected error for invalid Notice level")
	}
}

func TestDecodeStateInvalidState(t *testing.T) {
	if _, err := DecodeState([]byte(`{"state":"paused"}`)); err == nil {
		t.Fatal("expected error for invalid State.State")
	}
}

func TestDecodeHelloMalformed(t *testing.T) {
	if _, err := DecodeHello([]byte(`not json`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeExitMalformed(t *testing.T) {
	if _, err := DecodeExit([]byte(`not json`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeNoticeMalformed(t *testing.T) {
	if _, err := DecodeNotice([]byte(`not json`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeStateMalformed(t *testing.T) {
	if _, err := DecodeState([]byte(`not json`)); err == nil {
		t.Fatal("expected error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errWireTestBoom }

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errWireTestBoom }

var errWireTestBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

func TestEncodeToWriteError(t *testing.T) {
	if err := EncodeTo(errWriter{}, OpPing, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeFromReadError(t *testing.T) {
	if _, _, err := DecodeFrom(errReader{}); err == nil {
		t.Fatal("expected error")
	}
}
