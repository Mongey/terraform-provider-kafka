package kafka

import (
	"io/ioutil"
	"testing"
)

func loadFile(t *testing.T, file string) string {
	fb, err := ioutil.ReadFile(file)
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
				clientCert:          "../secrets/terraform-cert.pem",
				clientKey:           "../secrets/terraform-with-passphrase.pem",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "confluent",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files",
			args: args{
				clientCert:          "../secrets/terraform-cert.pem",
				clientKey:           "../secrets/terraform.pem",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files without passphrase",
			args: args{
				clientCert:          "../secrets/terraform-cert.pem",
				clientKey:           "../secrets/terraform.pem",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "wrong",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key content without passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/terraform-cert.pem"),
				clientKey:           loadFile(t, "../secrets/terraform.pem"),
				caCert:              loadFile(t, "../secrets/ca.crt"),
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/terraform-cert.pem"),
				clientKey:           loadFile(t, "../secrets/terraform-with-passphrase.pem"),
				caCert:              loadFile(t, "../secrets/ca.crt"),
				clientKeyPassphrase: "confluent",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/terraform-cert.pem"),
				clientKey:           "../secrets/terraform-with-passphrase.pem",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "confluent",
			},
			wantErr: false,
		},
		{
			name: "encrypted cert content without passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/terraform-cert.pem"),
				clientKey:           "../secrets/terraform-with-passphrase.pem",
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
