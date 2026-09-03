package wire

import "testing"

func BenchmarkOutputEncode(b *testing.B) {
	payload := make([]byte, 32*1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(OpOutput, payload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOutputDecode(b *testing.B) {
	frame, err := Encode(OpOutput, make([]byte, 32*1024))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := Decode(frame); err != nil {
			b.Fatal(err)
		}
	}
}
