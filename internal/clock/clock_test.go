package clock

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		sec  int
		want string
	}{
		{0, "00:00:00"},
		{59, "00:00:59"},
		{60, "00:01:00"},
		{3600, "01:00:00"},
		{8261, "02:17:41"},
		{15074, "04:11:14"},
		{100 * 3600, "100:00:00"},
	}
	for _, tt := range tests {
		if got := Format(tt.sec); got != tt.want {
			t.Errorf("Format(%d) = %q, want %q", tt.sec, got, tt.want)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "4:11:14", want: 15074},
		{in: "02:17:41", want: 8261},
		{in: "1:02", want: 62},
		{in: "7", want: 7},
		{in: "0", want: 0},
		{in: "90", want: 90},
		{in: "75:00", want: 4500},
		{in: "100:00:00", want: 360000},
		{in: "", wantErr: true},
		{in: "1::2", wantErr: true},
		{in: ":30", wantErr: true},
		{in: "1:", wantErr: true},
		{in: "a:00", wantErr: true},
		{in: "-5", wantErr: true},
		{in: "1:-5", wantErr: true},
		{in: "+5", wantErr: true},
		{in: "1:60:00", wantErr: true},
		{in: "1:00:60", wantErr: true},
		{in: "1:60", wantErr: true},
		{in: "1:2:3:4", wantErr: true},
		{in: " 7", wantErr: true},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Parse(%q) = %d, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	secs := []int{0, 1, 59, 60, 61, 3599, 3600, 8261, 15074, 99*3600 + 59*60 + 59, 360000}
	for _, sec := range secs {
		got, err := Parse(Format(sec))
		if err != nil {
			t.Errorf("Parse(Format(%d)) error: %v", sec, err)
			continue
		}
		if got != sec {
			t.Errorf("Parse(Format(%d)) = %d", sec, got)
		}
	}
}
