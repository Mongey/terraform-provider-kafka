package kafka

import (
	"crypto/rand"
	"fmt"
	"log"
	"strings"

	"github.com/IBM/sarama"
)

type UserScramCredential struct {
	Name       string
	Mechanism  sarama.ScramMechanismType
	Iterations int32
	Password   []byte
}

func (usc UserScramCredential) String() string {
	return strings.Join([]string{usc.Name, usc.Mechanism.String(), fmt.Sprint(usc.Iterations)}, "|")
}

func (usc UserScramCredential) ID() string {
	return strings.Join([]string{usc.Name, usc.Mechanism.String()}, "|")
}

type UserScramCredentialMissingError struct {
	msg string
}

func (e UserScramCredentialMissingError) Error() string { return e.msg }

const (
	saltSize = 64
)

func (c *Client) UpsertUserScramCredential(userScramCredential UserScramCredential) error {
	log.Printf("[INFO] Upserting user scram credential %v", userScramCredential)
	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return err
	}

	upsert, err := prepareUpsert(userScramCredential)
	if err != nil {
		return err
	}

	results, err := admin.UpsertUserScramCredentials([]sarama.AlterUserScramCredentialsUpsert{upsert})
	if err != nil {
		return err
	}

	for _, res := range results {
		if res.ErrorCode != sarama.ErrNoError {
			return res.ErrorCode
		}
	}

	return nil
}

func (c *Client) DescribeUserScramCredential(username string, mechanism string) (*UserScramCredential, error) {
	log.Printf("[INFO] Describing user scram credential %s", username)
	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return nil, err
	}

	results, err := admin.DescribeUserScramCredentials([]string{username})
	if err != nil {
		return nil, err
	}

	num := len(results)
	if num != 1 {
		return nil, fmt.Errorf("got %d results (expected 1) when describing user scram credential %s", num, username)
	}

	res := results[0]
	if res.ErrorCode == 91 { // RESOURCE_NOT_FOUND
		msg := fmt.Sprintf("User scram credential %s could not be found", username)
		return nil, UserScramCredentialMissingError{msg: msg}
	}
	if res.ErrorCode != sarama.ErrNoError {
		return nil, fmt.Errorf("error describing user scram credential %s: %s", username, res.ErrorCode)
	}
	for _, info := range res.CredentialInfos {
		if info.Mechanism.String() == mechanism {
			r := UserScramCredential{
				Name:       username,
				Mechanism:  info.Mechanism,
				Iterations: info.Iterations,
			}
			return &r, nil
		}
	}

	msg := fmt.Sprintf("User scram credential %s with mechanism %s could not be found", username, mechanism)
	return nil, UserScramCredentialMissingError{msg: msg}
}

func (c *Client) DeleteUserScramCredential(userScramCredential UserScramCredential) error {
	log.Printf("[INFO] Deleting user scram credential %v", userScramCredential)
	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return err
	}

	delete := prepareDelete(userScramCredential)
	results, err := admin.DeleteUserScramCredentials([]sarama.AlterUserScramCredentialsDelete{delete})
	if err != nil {
		return err
	}

	for _, res := range results {
		if res.ErrorCode != sarama.ErrNoError {
			return res.ErrorCode
		}
	}

	return nil
}

func prepareUpsert(userScramCredential UserScramCredential) (sarama.AlterUserScramCredentialsUpsert, error) {
	var ret sarama.AlterUserScramCredentialsUpsert
	ret.Name = userScramCredential.Name
	ret.Mechanism = userScramCredential.Mechanism
	ret.Iterations = userScramCredential.Iterations
	ret.Password = userScramCredential.Password
	salt, err := generateRandomBytes(saltSize)
	ret.Salt = append(ret.Salt, salt...)
	return ret, err
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func prepareDelete(userScramCredential UserScramCredential) sarama.AlterUserScramCredentialsDelete {
	var ret sarama.AlterUserScramCredentialsDelete
	ret.Name = userScramCredential.Name
	ret.Mechanism = userScramCredential.Mechanism
	return ret
}
