package config

import "testing"

func TestServiceURLNormalizesRenderHostport(t *testing.T) {
	cases := []struct{ input, want string }{
		{input: "tenant-service:10000", want: "http://tenant-service:10000"},
		{input: "https://service.example/", want: "https://service.example"},
		{input: " http://localhost:8082/// ", want: "http://localhost:8082"},
		{input: "", want: ""},
	}
	for _, testCase := range cases {
		if got := ServiceURL(testCase.input); got != testCase.want {
			t.Errorf("ServiceURL(%q)=%q want %q", testCase.input, got, testCase.want)
		}
	}
}

func TestRequireProductionEnv(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("PRIVATE_TOKEN", "")
	if err := RequireProductionEnv("PRIVATE_TOKEN"); err != nil {
		t.Fatalf("development optional value rejected: %v", err)
	}

	t.Setenv("ENVIRONMENT", " Production ")
	if err := RequireProductionEnv("PRIVATE_TOKEN"); err == nil {
		t.Fatal("production missing value must fail closed")
	}

	t.Setenv("PRIVATE_TOKEN", "configured")
	if err := RequireProductionEnv("PRIVATE_TOKEN"); err != nil {
		t.Fatalf("production configured value rejected: %v", err)
	}
}
