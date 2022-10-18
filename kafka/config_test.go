package kafka

import (
	"os"
	"testing"
)

func loadFile(t *testing.T, file string) string {
	fb, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("[ERROR] Error reading file %s", err)
	}
	return string(fb)
}

func Test_newTLSConfig(t *testing.T) {
	type args struct {
		clientCert          string
		clientKey           string
		caCert              string
		clientKeyPassphrase string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "no special certs",
			args: args{
				clientCert:          "",
				clientKey:           "",
				caCert:              "",
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "encrypted key files",
			args: args{
				clientCert:          "../secrets/client.pem",
				clientKey:           "../secrets/client.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "test-pass",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files",
			args: args{
				clientCert:          "../secrets/client.pem",
				clientKey:           "../secrets/client-no-password.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files without passphrase",
			args: args{
				clientCert:          "../secrets/client.pem",
				clientKey:           "../secrets/client-no-password.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "wrong",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key content without passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           loadFile(t, "../secrets/client-no-password.key"),
				caCert:              loadFile(t, "../secrets/ca.crt"),
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           loadFile(t, "../secrets/client.key"),
				caCert:              loadFile(t, "../secrets/ca.crt"),
				clientKeyPassphrase: "test-pass",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           "../secrets/client.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "test-pass",
			},
			wantErr: false,
		},
		{
			name: "encrypted cert content without passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           "../secrets/client.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTLSConfig(tt.args.clientCert, tt.args.clientKey, tt.args.caCert, tt.args.clientKeyPassphrase)
			if (err != nil) != tt.wantErr {
				t.Errorf("newTLSConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
