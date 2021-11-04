package kafka

import (
	"fmt"
	"log"
	"crypto/rand"

	"github.com/Shopify/sarama"
)
const (
	saltSize = 64
)

type UserScramCredentialMissingError  struct {
	msg string
}

func (e UserScramCredentialMissingError ) Error() string { return e.msg }

type UserScramCredential struct {
	Name           string
	Mechanism      sarama.ScramMechanismType
	Iterations     int32
	Password	   string
}

func (u UserScramCredential) String() string {
	return u.Name
}

func (u UserScramCredential) ID() string {
	return u.Name
}


func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func prepareUpsert(userScramCredential UserScramCredential) (*sarama.AlterUserScramCredentialsUpsert, error) {
	var ret sarama.AlterUserScramCredentialsUpsert
	ret.Name = userScramCredential.Name
	ret.Mechanism = userScramCredential.Mechanism
	ret.Password = []byte(userScramCredential.Password)
	salt, err := generateRandomBytes(saltSize)
	ret.Salt = append(ret.Salt, salt...)
	return &ret, err
}

func prepareDelete(userScramCredential UserScramCredential) *sarama.AlterUserScramCredentialsDelete {
	var ret sarama.AlterUserScramCredentialsDelete
	ret.Name = userScramCredential.Name
	ret.Mechanism = userScramCredential.Mechanism
	return &ret
}


func (c *Client) UpsertUserScramCredential(userScramCredential UserScramCredential) error {
	log.Printf("[INFO] Upserting user scram credential")
	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return err
	}
	upsert, err := prepareUpsert(userScramCredential)

	if err != nil {
		return err
	}

	results, err := admin.UpsertUserScramCredentials([]sarama.AlterUserScramCredentialsUpsert{*upsert})

	for _, res := range results {
		if res.ErrorCode != sarama.ErrNoError {
			return res.ErrorCode
		}
	}

	return nil
}

func (c *Client) DeleteUserScramCredential(userScramCredential UserScramCredential) error {
	log.Printf("[INFO] Deleting user scram credential")
	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return err
	}
	delete := prepareDelete(userScramCredential)
	results, err := admin.DeleteUserScramCredentials([]sarama.AlterUserScramCredentialsDelete{*delete})

	for _, res := range results {
		if res.ErrorCode != sarama.ErrNoError {
			return res.ErrorCode
		}
	}

	return nil
}


func (c *Client) DescribeUserScramCredential(credentialName string) (*UserScramCredential, error) {
	log.Printf("[INFO] Describing User Scram Credential")
	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return nil, err
	}

	results, err := admin.DescribeUserScramCredentials([]string{credentialName})
	if err != nil {
		return nil, err
	}

	if len(results) < 1 {
		return nil, UserScramCredentialMissingError{msg: fmt.Sprintf("user scram credential %s could not be found", credentialName)}
	}

	res := []UserScramCredential{}
	for _, result := range results {
		if result.ErrorCode != sarama.ErrNoError {
			return nil, fmt.Errorf("Error describing user scram credential '%s': %s", credentialName, *result.ErrorMessage)
		}
		r := UserScramCredential{
			Name: result.User,
			Mechanism: result.CredentialInfos[0].Mechanism,
			Iterations: result.CredentialInfos[0].Iterations,
		}
		res = append(res, r)
	}

	return &res[0], err
}
