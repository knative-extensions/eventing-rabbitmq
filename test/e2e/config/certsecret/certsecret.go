/*
Copyright 2022 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certsecret

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	b64 "encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"knative.dev/eventing-rabbitmq/test/e2e/config/rabbitmq"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"

	kubeClient "knative.dev/pkg/client/injection/kube/client"
)

//go:embed "*.yaml"
var yamls embed.FS

const (
	interval = 1 * time.Second
	timeout  = 5 * time.Minute
)

// Install will generate a CA and sign a new certificate with it. It will then populate the values into secrets in the yaml file.
func Install(ctx context.Context, t feature.T) {
	namespace := environment.FromContext(ctx).Namespace()
	args, err := createCerts(namespace)
	if err != nil {
		t.Fatal(err)
	}

	args["tlsSecretName"] = rabbitmq.TLS_SECRET_NAME
	args["caSecretName"] = rabbitmq.CA_SECRET_NAME

	if _, err := manifest.InstallYamlFS(ctx, yamls, args); err != nil {
		t.Fatal(err)
	}

	for _, secretName := range []string{rabbitmq.TLS_SECRET_NAME, rabbitmq.CA_SECRET_NAME} {
		if err = wait.PollImmediate(interval, timeout, func() (bool, error) {
			_, err = kubeClient.Get(ctx).CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					t.Log(namespace, secretName, "not found", err)
					// keep polling
					return false, nil
				}
				return false, err
			}
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func createCerts(namespace string) (map[string]interface{}, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			CommonName: "Eventing RabbitMQ E2E Tests CA",
			Locality:   []string{"$$$$"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName: "Eventing RabbitMQ E2E Tests",
			Locality:   []string{"$$$$"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:     []string{fmt.Sprintf("rabbitmqc.%s.svc", namespace), fmt.Sprintf("rabbitmqc.%s.svc.cluster.local", namespace)},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	args := map[string]interface{}{
		"tlsCert": b64.StdEncoding.EncodeToString(certPEM.Bytes()),
		"tlsKey":  b64.StdEncoding.EncodeToString(certPrivKeyPEM.Bytes()),
		"caCert":  b64.StdEncoding.EncodeToString(caPEM.Bytes()),
	}

	return args, nil
}
