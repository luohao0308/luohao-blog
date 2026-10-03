package service

import "testing"

// 伪造的最左跳不能成为限流键：反代链每跳都在右侧追加，最右才是可信代理
// 看到的真实客户端地址（2026-10 review P1 的回归锚点）。
func TestRightmostForwardedIP(t *testing.T) {
	cases := []struct {
		name string
		xff  string
		want string
	}{
		{"single hop", "203.0.113.7", "203.0.113.7"},
		{"spoofed first hop", "9.9.9.9, 203.0.113.7", "203.0.113.7"},
		{"multiple hops", "10.0.0.1, 10.0.0.2, 203.0.113.7", "203.0.113.7"},
		{"trailing comma", "203.0.113.7, ", "203.0.113.7"},
		{"extra whitespace", "  9.9.9.9 ,  203.0.113.7  ", "203.0.113.7"},
		{"empty", "", ""},
		{"only commas", ", ,", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rightmostForwardedIP(tc.xff); got != tc.want {
				t.Fatalf("rightmostForwardedIP(%q) = %q, want %q", tc.xff, got, tc.want)
			}
		})
	}
}
