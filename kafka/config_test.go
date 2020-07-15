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
				clientCert:          "../secrets/kafkacat-ca1-signed.pem",
				clientKey:           "../secrets/kafkacat-raw-private-key-passphrase.pem",
				caCert:              "../secrets/snakeoil-ca-1.crt",
				clientKeyPassphrase: "confluent",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files",
			args: args{
				clientCert:          "../secrets/kafkacat-ca1-signed.pem",
				clientKey:           "../secrets/kafkacat-raw-private-key.pem",
				caCert:              "../secrets/snakeoil-ca-1.crt",
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files without passphrase",
			args: args{
				clientCert:          "../secrets/kafkacat-ca1-signed.pem",
				clientKey:           "../secrets/kafkacat-raw-private-key.pem",
				caCert:              "../secrets/snakeoil-ca-1.crt",
				clientKeyPassphrase: "wrong",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key content without passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/kafkacat-ca1-signed.pem"),
				clientKey:           loadFile(t, "../secrets/kafkacat-raw-private-key.pem"),
				caCert:              loadFile(t, "../secrets/snakeoil-ca-1.crt"),
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/kafkacat-ca1-signed.pem"),
				clientKey:           loadFile(t, "../secrets/kafkacat-raw-private-key-passphrase.pem"),
				caCert:              loadFile(t, "../secrets/snakeoil-ca-1.crt"),
				clientKeyPassphrase: "confluent",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/kafkacat-ca1-signed.pem"),
				clientKey:           "../secrets/kafkacat-raw-private-key-passphrase.pem",
				caCert:              "../secrets/snakeoil-ca-1.crt",
				clientKeyPassphrase: "confluent",
			},
			wantErr: false,
		},
		{
			name: "encrypted cert content without passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/kafkacat-ca1-signed.pem"),
				clientKey:           "../secrets/kafkacat-raw-private-key-passphrase.pem",
				caCert:              "../secrets/snakeoil-ca-1.crt",
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
