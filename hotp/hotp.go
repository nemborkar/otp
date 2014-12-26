/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package hotp

import (
	"github.com/pquerna/otp"

	"crypto/hmac"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
)

const debug = false

// Validates that input is a valid HOTP given
// the secret and a counter. A shortcut for ValidateCustom.
func Validate(input string, counter uint64, secret string) bool {
	rv, _ := ValidateCustom(
		input,
		counter,
		secret,
		ValidateOpts{
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)
	return rv
}

type ValidateOpts struct {
	// Digits as part of the input. Defaults to 6.
	Digits otp.Digits
	// Algorithm to use for HMAC. Defaults to SHA1.
	Algorithm otp.Algorithm
}

func ValidateCustom(input string, counter uint64, secret string, opts ValidateOpts) (bool, error) {
	input = strings.TrimSpace(input)

	switch opts.Digits {
	case otp.DigitsSix:
		if len(input) != 6 {
			return false, otp.ValidateInputInvalidLength6
		}
	case otp.DigitsEight:
		if len(input) != 8 {
			return false, otp.ValidateInputInvalidLength8
		}
	default:
		panic("unsupported Digits value.")
	}

	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return false, otp.ValidateSecretInvalidBase32
	}

	buf := make([]byte, 8)
	mac := hmac.New(opts.Algorithm.Hash, secretBytes)
	binary.BigEndian.PutUint64(buf, counter)
	if debug {
		fmt.Printf("counter=%v\n", counter)
		fmt.Printf("buf=%v\n", buf)
	}

	mac.Write(buf)
	sum := mac.Sum(nil)

	// "Dynamic truncation" in RFC 4226
	// http://tools.ietf.org/html/rfc4226#section-5.4
	offset := sum[len(sum)-1] & 0xf
	value := int64(((int(sum[offset]) & 0x7f) << 24) |
		((int(sum[offset+1] & 0xff)) << 16) |
		((int(sum[offset+2] & 0xff)) << 8) |
		(int(sum[offset+3]) & 0xff))

	l := opts.Digits.Legnth()
	mod := int32(value % int64(math.Pow10(l)))

	if debug {
		fmt.Printf("offset=%v\n", offset)
		fmt.Printf("value=%v\n", value)
		fmt.Printf("mod'ed=%v\n", mod)
	}

	otpstr := opts.Digits.Format(mod)

	if subtle.ConstantTimeCompare([]byte(otpstr), []byte(input)) == 1 {
		return true, nil
	}

	return false, nil
}

// Options for .Generate()
type GenerateOpts struct {
	// Name of the issuing Organization/Company.
	Issuer string
	// Name of the User's Account (eg, email address)
	AccountName string
	// Size in size of the generated Secret. Defaults to 10 bytes.
	SecretSize uint
	// Digits to request. Defaults to 6.
	Digits otp.Digits
	// Algorithm to use for HMAC. Defaults to SHA1.
	Algorithm otp.Algorithm
}

// Generates a new HOTP Key.
func Generate(opts GenerateOpts) (*otp.Key, error) {
	// url encode the Issuer/AccountName
	if opts.Issuer == "" {
		return nil, otp.GenerateMissingIssuer
	}

	if opts.AccountName == "" {
		return nil, otp.GenerateMissingAccountName
	}

	if opts.SecretSize == 0 {
		opts.SecretSize = 10
	}

	// otpauth://totp/Example:alice@google.com?secret=JBSWY3DPEHPK3PXP&issuer=Example

	v := url.Values{}
	secret := make([]byte, opts.SecretSize)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, err
	}

	v.Set("secret", base32.StdEncoding.EncodeToString(secret))
	v.Set("issuer", opts.Issuer)
	v.Set("algorithm", opts.Algorithm.String())
	v.Set("digits", opts.Digits.String())

	u := url.URL{
		Scheme:   "otpauth",
		Host:     "hotp",
		Path:     "/" + opts.Issuer + ":" + opts.AccountName,
		RawQuery: v.Encode(),
	}

	return otp.NewKeyFromURL(u.String())
}
