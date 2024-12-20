package payments

import (
	"fmt"
	"net/url"

	"github.com/gagliardetto/solana-go"
)

var metaplexToken = solana.MustPublicKeyFromBase58("METAewgxyPbgwsseH8T16a39CQ5VyVxZi9zXiDPY18m")

const (
	paymentLabel = "Aura Gateway"
)

func GenerateSolanaPayPaymentLink(recipient solana.PublicKey, amount, paymentType string) (string, error) {
	//ref := make([]byte, 32)
	//_, err := rand.Read(ref)
	//if err != nil {
	//	return "", fmt.Errorf("rand.Read: %w", err)
	//}
	//referenceKey := solana.PublicKeyFromBytes(ref)
	referenceKey, err := solana.NewRandomPrivateKey()
	if err != nil {
		return "", fmt.Errorf("NewRandomPrivateKey: %w", err)
	}
	u := url.URL{
		Scheme: "solana",
		Opaque: recipient.String(),
	}
	q := u.Query()
	q.Set("amount", amount)
	q.Set("spl-token", metaplexToken.String())
	q.Set("reference", referenceKey.PublicKey().String())
	q.Set("label", paymentLabel)
	q.Set("message", fmt.Sprintf("Payment type: %s", paymentType))
	u.RawQuery = q.Encode()

	return u.String(), nil
}
