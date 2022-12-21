package api

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// roleMap defines the accessible methods for each role
var roleMap = map[string][]string{
	"/job.JobManager/Start":  {"admin"},
	"/job.JobManager/Stop":   {"admin"},
	"/job.JobManager/Status": {"admin", "user"},
	"/job.JobManager/Output": {"admin", "user"},
}

// unaryInterceptor is a grpc inteceptor that authorizes access to the methods as listed in roleMap
func unaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	// get the peer information so we can parse the client certificate out of it
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("error reading peer information from context")
	}
	tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, errors.New("could not find peer authentication information")
	}
	// get the peer (client) certificate from tlsInfo
	peerCerts := tlsInfo.State.PeerCertificates
	if len(peerCerts) == 0 {
		return nil, errors.New("missing peer certificate")
	} else if len(peerCerts[0].Subject.Organization) == 0 {
		return nil, errors.New("no role set for certificate")
	}

	// find role from client certificate and check if it has access to the method.
	// I'm assuming just one role is set for simplicity, but in production this would support multiple roles
	role := peerCerts[0].Subject.Organization[0]
	if !isAuthorized(info.FullMethod, role) {
		return nil, fmt.Errorf("role %q is not unauthorized to execute %s", role, info.FullMethod)
	}

	return handler(ctx, req)
}

func isAuthorized(method, role string) bool {
	perms, ok := roleMap[method]
	if !ok {
		return false
	}
	for _, v := range perms {
		if role == v {
			return true
		}
	}
	return false
}
