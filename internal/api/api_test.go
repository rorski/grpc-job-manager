package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"testing"

	"github.com/rorski/grpc-job-manager/internal/job"
	"github.com/rorski/grpc-job-manager/worker"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var conf = Config{
	Host:        "localhost",
	Port:        31234,
	Certificate: string(serverCert),
	Key:         string(serverKey),
	CA:          string(caCert),
}

func TestAdminHasAuth(t *testing.T) {
	for method := range roleMap {
		authorized := isAuthorized(method, "admin")
		assert.True(t, authorized)
	}
}

func TestUserHasStatusAndOutputAuth(t *testing.T) {
	assert.True(t, isAuthorized("/job.JobManager/Status", "user"))
	assert.True(t, isAuthorized("/job.JobManager/Output", "user"))
}

func TestUserNotHaveStartAndStopAuth(t *testing.T) {
	assert.False(t, isAuthorized("/job.JobManager/Start", "user"))
	assert.False(t, isAuthorized("/job.JobManager/Stop", "user"))
}

// TestAuthzStartAsAdmin tests starting a "ps" job with an admin role (from the client cert)
func TestAuthzStartAsAdmin(t *testing.T) {
	// load server credentials and start a grpc server
	serverCreds, err := loadServerCreds()
	assert.NoError(t, err)

	s, lis, err := newGrpcServer(conf, serverCreds)
	defer s.Stop()
	job.RegisterJobManagerServer(s, &jobManagerServer{Worker: *worker.New()})
	go func() {
		defer lis.Close()
		err = s.Serve(lis)
		assert.NoError(t, err)
	}()

	// use the "admin" cert/key below to try to start a job
	userCreds, err := loadClientCreds(caCert, "admin")
	assert.NoError(t, err)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", conf.Host, conf.Port), grpc.WithTransportCredentials(userCreds))
	assert.NoError(t, err)
	defer conn.Close()

	// try to start a "ps" command as the admin role
	jobClient := job.NewJobManagerClient(conn)
	res, err := jobClient.Start(context.Background(), &job.StartRequest{Cmd: "ps"})
	assert.NoError(t, err)
	assert.NotEmpty(t, res.Uuid)
}

// TestAuthzStartAsUser tests starting a "ps" job with an user role (from the client cert)
func TestAuthzStartAsUser(t *testing.T) {
	serverCreds, err := loadServerCreds()
	assert.NoError(t, err)

	s, lis, err := newGrpcServer(conf, serverCreds)
	defer s.Stop()
	job.RegisterJobManagerServer(s, &jobManagerServer{Worker: *worker.New()})
	go func() {
		defer lis.Close()
		err = s.Serve(lis)
		assert.NoError(t, err)
	}()

	// use the "user" cert/key below to try to start a job
	userCreds, err := loadClientCreds(caCert, "user")
	assert.NoError(t, err)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", conf.Host, conf.Port), grpc.WithTransportCredentials(userCreds))
	assert.NoError(t, err)
	defer conn.Close()

	// try to start a "ps" command as the admin role
	jobClient := job.NewJobManagerClient(conn)
	res, err := jobClient.Start(context.Background(), &job.StartRequest{Cmd: "ps"})
	assert.NotNil(t, err)
	assert.Nil(t, res)
}

func loadClientCreds(ca []byte, role string) (credentials.TransportCredentials, error) {
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to add CA's certificate")
	}
	var clientCert, clientKey []byte
	if role == "user" {
		clientCert = clientUserCert
		clientKey = clientUserKey
	} else if role == "admin" {
		clientCert = clientAdminCert
		clientKey = clientAdminKey
	} else {
		return nil, fmt.Errorf("role %s not found", role)
	}

	certs, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, fmt.Errorf("error loading x509 key pair: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certs},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS13,
	}
	return credentials.NewTLS(tlsConfig), nil
}

func loadServerCreds() (credentials.TransportCredentials, error) {
	cert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load x509 key pair: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA cert to pool: %v", err)
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // require client auth (i.e., mTLS)
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

var caCert = []byte(`-----BEGIN CERTIFICATE-----
MIIE3TCCAsWgAwIBAgICB+YwDQYJKoZIhvcNAQELBQAwADAeFw0yMjA5MTYwNDA3
MTBaFw0yMzA5MTYwNDA3MTBaMAAwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIK
AoICAQDtOcPVkpVwSl0MJt9jQT6SgrVnFMGgHjyub0XAl4/vH5vYSlG2XF8NsCm/
AK1F4Ht8+hMvVLDq8DxDqtRXqRVdo5iALS8zudvoHMaiv5bDP2GH+OCnu8Jf5vNs
7yCJxyu6c+y/fG6X/3AeZ04kamnHhN1VhDWu8Zqr4JZ/6iaJrkXYbh0VteefU5FM
YYG34N/licesgzjI1NaGL5WumR98WZLyB83S60X7mc2m2cUFhratxwuGHFi/n+KE
LxzRXGRkHl2nfxGrg7TKNZ6DnGc6Vxvbs7X68SwUcBu99Dlt9qQrox2H3axr4VjX
FfpiO+UN2aWq+zupeyNw1cd1SRST3BMU2gOPVaF74t66BNCDoNJXfsQR7tM5MZuz
sFaJ+WYqdr8QebLpPdp8jS2OMQEhwy6SBsIJ+bqghfG2lm9+qRnxMYAqDoOGtW3k
HVCN7QJL5E6MwwAYdyIkbtvIPEOCOtQ4hxBwvXOk448GGrWRnqR2I8nGUIIMqB34
6+OBCdXLkDmWdnjmexHXgWXg/cbonM3lKGehwH/YkHifSRE82edYyHzgGQiCes9W
zDCvSiddQ4+jdPx0Qn8BM1CqI5kTuqy9LPdVxiQIBlqxZ7cLasn9PlkguKSFj096
fpbYNzrfOe4MssCm9ofjJq/tvv4kjrI4qdxQ8G4ElYdtfAs7JwIDAQABo2EwXzAO
BgNVHQ8BAf8EBAMCAoQwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMA8G
A1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFPtocYs+L+CK77tF1T3Sf1EMKSBtMA0G
CSqGSIb3DQEBCwUAA4ICAQCOoS9DExCVLrYJBL7XyBmaUTUc6MIbJjAkTotvC0rI
DxsNQOj/KJm5mgDhyjLxvqEw7A/Q0jmEiVlqhx1cEOVbpdlVGXT+3lu60cHTDTvN
6oZfQ6tHKub7fkeu9tNO5/vBDpBhDSeGqpk5bwHhp9ASDxD7wJNgN7M4VDrYZfoP
ZbIHuA2KDtxq+sj5RVoWphwzBtot+M8x22mxVk4eB4/GAsis/98HgKBdsgIHubET
AnoBSgP7e9JxCDzJ362CaI/gzvSBhZgRs7qLmZQTI2PzXwfPR+ka2zeGzQ9M1DiP
bDjM/UVFnTnDqgmJMnggECfHyXw91qvIYaW8VE3v4j/Ji5Gslcj/DcrWIqwXeAU1
1n9+nYDdrC2zjYBbhGPGllRekAiUN8wM/cKsHLc58xw55fVGpNU2A+oKwp1Uzj8u
CJEJtzi8tdlUPil0k2Pj4C0rgG8vkMgMNHbUF06btynGA5PbzNpe3kvLZKrsAL2B
oN7UDrWlPYiUFe929/uUzwzjz4lgBzgJEkkhySS5RvlusNm7mnZ+ZSZZ63Wfgwrp
BAPK4N0ZEatml45oVU0MAv3zQpRjBvcPT7xUlmFTpRcF27IgpxGyhByKcRwZt8rM
aRQieuYD8VKo/IcMafGX0F+gQUF96eaxzNXNkzATG7JzAoCBOIsE5Wh83hZvuBbR
Bw==
-----END CERTIFICATE-----`)
var caKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
var serverCert = []byte(`-----BEGIN CERTIFICATE-----
MIIFBTCCAu2gAwIBAgICB+YwDQYJKoZIhvcNAQELBQAwADAeFw0yMjA5MTYwNDA3
MTJaFw0yMzA5MTYwNDA3MTJaMCExDjAMBgNVBAoTBWFkbWluMQ8wDQYDVQQDEwZz
ZXJ2ZXIwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCjS+UxVyTS4aO4
E6sXRm4qdBnAc7E126W80dcNZG+7M6UcANaR9VIPz83N3wV1mM+rX3mLcvCIN5/o
MRO7t+VXnan/ai5Ea2g6W1pFDnC11JOGcAqOeK1jTkFfer9+8xJ2kEVQpQBvBG45
DmVrbL/KZnqSaXbVthx4RRDbz0RkBhmMNj9Hh3qVV9YpaDjAwGxEvZHGI2oi0jly
Di4m4OvR/G4GLC24LK8IqhwJCA27azalvvfznKLlrOEZr8uXrsGsshBK7Do5N80z
qCjbOTpA2Q5R3t+9f77NxbcRlLxvP8Fv6s6v0WIpyngs0hNqw6WFuFw23mxMl6Qy
KyKYz7GbfypktW1eEGdexUqlP5X8VX18mS67GAsC34Z2Nu+mrqrSnDj9uuCZgI4U
7j4iGRLdJQqL6gGxB80y0Ipa4dH9ey3EGGpAuxwGqniKqmYrzadKY34xSYzFAW0y
kS+ebv+d1qqDDUzCoQH06Yg0lmLDSslrcpYU1TYcEdR3mzp0bUulmmJIKI8e5VwQ
AXIM9lRSCJR+PU0n+a1uHT0rqixbz8cfJWbQSPXrADLz32acF5bBUAPUWdTGT+5R
ophtau7zKRz8RwzjE3yml8kZEojt5bMS0+sHPBODzIJPaCbkn6dd5gc1FucQBbVU
m3YY4yJy25NVwlsJGe0t56p8FfhM2wIDAQABo2gwZjAOBgNVHQ8BAf8EBAMCB4Aw
HQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMB8GA1UdIwQYMBaAFPtocYs+
L+CK77tF1T3Sf1EMKSBtMBQGA1UdEQQNMAuCCWxvY2FsaG9zdDANBgkqhkiG9w0B
AQsFAAOCAgEAHtBSIf9rLaKjee7k/L3GWgb1vyh8robS0HhHmCRWcuJ0EJjPIW3M
LRXMTIlZ2HShDUxrEjPSrgT633hvSSwccBQ13L41SiIjaVtkghgzAAHaXnDbMv2r
1kupXBlIXEkCO9yHVx7Q6PAK2ImVAt1w0/ZOnxOfc7tcso8/RXLHpVTdH1rnzgRE
3uykmzo8YxBZAuDg0PPYv39ZMz5tmXfgVcIF3vuLJKciefkwPIEarXkYi+ft/3/b
+3wjcoHZoc5XvVshStVNT3CMqoj2m5/bP1vlMcCWSuYnRVeVqLedHJdDvMTRo8Ig
qm5ZQUT2akPmg0UyOXv7AGvUvD1YAEM4sOIDDYiEd5nsF8MF8gw7xf1njLMpgJE6
Kov7153/zqLz2WR5/vWh4b4nhQ3CrcA7VwaY7+qgG9hmjSnoZ4KohXS1Ci6c3+ny
yz5T6aV24KwZoemWSObKbH/vdWeShVGbSwu8zHvpjK3HOrrRhnMYiYVQtIfDrfSy
VuVrbkOA//+YP2jAKEzPgCgIs1CA0R4c18GcCDwSSBf4hJ+a3FP78B8SMKqZlrcM
35pGEpDd1Gsccz7b9Lz4JoDNligfBazOcP96vzfwvZBWpwkp05n/mMiAFTeaFEf7
2Pt20HkDFjMTuPY4RuxfmiKCtYgJM0vtLGULo92+TwhRTcRQmjRA0i8=
-----END CERTIFICATE-----`)
var serverKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
var clientAdminCert = []byte(`-----BEGIN CERTIFICATE-----
MIIFCzCCAvOgAwIBAgICB+YwDQYJKoZIhvcNAQELBQAwADAeFw0yMjA5MTYwNDA3
MTRaFw0yMzA5MTYwNDA3MTRaMCcxDjAMBgNVBAoTBWFkbWluMRUwEwYDVQQDDAxj
bGllbnRfYWRtaW4wggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCYMkks
0yXu6o7DjgtoIPY4zbZCh3yWXDewOweZzVIJrL2dRXNgUPMYI5aPGuYk6AM/3rwQ
Jb47qE6a9dQ7lFiLNvLeG5iRcR/DwUela9ezLE3tO8RGybk9nWY1ZS4SIZBRCE8M
5o3q6Yu6N/af3xhrTinFTFZx9MULw4uNiVhrZGkWQmRPYmGtsMqMLoOt+JYuc+RR
kbwLqw9i4+sJu/vuY3eI3Jplxre/3h7uK3f6ASqOcPHSLtUwTFEmmtQtHj0DTW1Q
UNcbPHr8N4dx65K56a5SXQCgya+ZU74VN1Z3cnWg6XEkCqZXc9ChnhZDjtsIP0Fl
mSFKnCA4IptXfecc+NrPk1PHbFK3X8RFr41oy7+3FCBSV1qhZpV5WkFgP1pum+2E
fAbWvdRLyzgf0kCvn5HAXib3jTSGwI2c3uye66wZ66YVU2ZFhQAyMQKw+ZJK6SYy
lfoe8PUTFkvTuprtskEg+hfXhpVB5o5RIu4dKPi5kTDqUZLXk2a9ZIDWvPpvfmYQ
+9QefGEmJO9gotRB32LkFDoFLdglXxEzQwRLJZPr33zNLBhS6K7RRk33yOR1xlkY
xqF53mBvwZWCy+7iyms7P/ZmQ7S0BOh1wlSkkMHun81N447H5gQmINSAXBJ9YXRI
iZjiQtHe0yZO4TVlyMmwd5mD/pw2nsLIXTpfnQIDAQABo2gwZjAOBgNVHQ8BAf8E
BAMCB4AwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMB8GA1UdIwQYMBaA
FPtocYs+L+CK77tF1T3Sf1EMKSBtMBQGA1UdEQQNMAuCCWxvY2FsaG9zdDANBgkq
hkiG9w0BAQsFAAOCAgEAQHKMChp7jHL5UMdMxwBFeve0gmtNsT8VIkuB3BIwr9xl
1eUZUvHjr/ED/FcxoFpbKYmCa7UrLi8X9sqHg5wIr2bKo4I8m2XEFFwr2Xummq69
N1JOhqOpWEd29ufe8OxbHMv3SM3GYzbJXn/Or0n/pSWJ2BCifxJtJ1csXTZf9Mp6
QhCOGuizY4dKea2EjrCkfEpDPD9zUt817KugI1yHVD16sq8KofU4cmGjXfv2IUxV
hq15T/K7hUBujPFscHOeyOn6r4RCmtDq8JO/EzyaQCwBUj53LmH0dDwIVkX08EBO
eFXEeLIOa1OHvKQ1c3Q3AlqjpNiqSwUUkxSJbncvqKiktoMd7/kvposXsBP9Y3yp
vYh1GyMxPvdMDqFMD44qx/f7+tmrwy/jVBAdpi6Izl4aPo8CF1xWBa2kOjaYeozq
ovNfnRz1VOEvzl+A3ZRTKMSiGUfqstLcjVggfd46ibXH/7z+sUwjqRwhgh6HJsm8
IWwI9mUj7bnb7MWddZmoVHVgRY0VtRUKLUKBkRx+L1s/LyNTDsniOhvQgViNZXNV
PEewMLYbyMsU/4l9OPms/f/sf299fZNNtB15fStd7SoXWhDHyvoUhgJhHu1qFgo3
vsYhn+kDVjHYu0PUdMkyIxGhnTVwvZMNRSRFFK+rIxwKJSmYXbSJyo5HJs/CQN4=
-----END CERTIFICATE-----`)
var clientAdminKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
var clientUserCert = []byte(`-----BEGIN CERTIFICATE-----
MIIFCTCCAvGgAwIBAgICB+YwDQYJKoZIhvcNAQELBQAwADAeFw0yMjA5MTYwNDA3
MTNaFw0yMzA5MTYwNDA3MTNaMCUxDTALBgNVBAoTBHVzZXIxFDASBgNVBAMMC2Ns
aWVudF91c2VyMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA67Kh5FOw
hQu7EOW5rVuwnzLwfOJFGa+gYqSmG0w+JfdieUOiR2WfeAV/TWLYI0Lk55GEiLaa
1C5ZB/MZ8KBVenfGmik9q40P4Z5yhQiHeKSMhwYJhsfazO4ICU0kSnLRIh9dghN/
i+ZjVPdtHsGC244v+D5M5uDaLv9KvSf+ImWiNh4t+A6P+IE/G8EP2wY0iu2vCCtK
2tE2XqQCBSEfLvqBc7SFUTpWutPZyad3HuGRs/EVvOqLbn22+DEMpdK7c0qA4kGu
BfuhJaqnDe3V6WLuMXMV9U0dteHx6Sjlv3jTxPnvL5BhtPECxIG8vHkkloX2mMKm
nHt8YWky5m6DJbkDtXxiPWDDL95lAzr79DomX2yVBm0J25sVQ7lcNmeoynjcVEoA
Y2Gtkj2CE/9wK8yjhTOp75n1/i79cxYeJaA5SiYRfxYp3rOxiCmcyXIwV1Yj2EiG
QmUwHj6n6PCwMwubH5TPO/D1Q0cwPv1UWgfrHsH5q3xmniMCqdxD+15H1ByGNIKk
55hIaVWugAIJ2txYusmhmEOc2oqmohEO9xaQmxiEpbSbSZU7JiqgsWuUVamHsdL6
gcixlaRcdsWiIRpU+DIl5pwnCfnj+9KDf+kk4phk56TAOEqHopTz+ROTCwMN3DXj
wb+BOwpC/fdglDQl5d2HwJgLugaitaSBv9sCAwEAAaNoMGYwDgYDVR0PAQH/BAQD
AgeAMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAfBgNVHSMEGDAWgBT7
aHGLPi/giu+7RdU90n9RDCkgbTAUBgNVHREEDTALgglsb2NhbGhvc3QwDQYJKoZI
hvcNAQELBQADggIBABmoO/B1tu9YUm/miWQhWVcebr45PIu/W/Lq8Fe7z4jkbB3W
ZSdQaK1L1Bq9scMHP962HusETa7JzTqSpfvrXnwR6jzuz+TWLvzcL5ytCBJ0M+4W
uutrluAOoO/GFTVPXbRIKyOyvuuKxUZcmrlRxkhzSUziRcJ3SvH2WIPYZFU6JUY2
8XAuR/jgqaIgHdpy8RfsaAeIGvd7cdwiBQwK5GHg7I+hqaoJPKArWf8EiPwGhfGG
y7cbBKzsHXKjNJlznc9csuCodyh0/xQKrl+JtBdUhwZC2UZGfcuhZsqtOdgTYGq5
rm7KF8XQJ0Q/xUkuhgLqZLEaDZ4vJE3K0O+JMHrMjbQuWTQkv99RLeO7/dj83/1+
2uEuaoXinhesg/aNg8q5pql4djal5/oAVdX9ljrZQvUtYsi80G+LvjB3NnLIk4ut
xcombuvdrLQxu2Od0bjuEqbsLwjhdO7bixfGfY2aAgo+JNF7vOKJovFl5igv6OxU
6p07vVCUt1AOR7S8LN7K7t+JQKT5ZtCMBvsI67FXmIC5IvbxpFpm7otkX3hPJbIC
94QzCQ4wtbpwt4O2ZFh96qW6Z6x/53SyMgmrXtBMZT8lY4BNkIT5gpyca36ZstzR
giihWtdNHg7WYx2kw2grFFFfc2tTAnhaY7k8x6PVuI59as3QGn3IiYbIkke8
-----END CERTIFICATE-----`)
var clientUserKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
