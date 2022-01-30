package federation

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/wasabee-project/Wasabee-Server/config"
	pb "github.com/wasabee-project/Wasabee-Server/federation/pb"
	"github.com/wasabee-project/Wasabee-Server/log"
)

type wafed struct {
	pb.UnimplementedWasabeeFederationServer
}

var (
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errInvalidToken    = status.Errorf(codes.Unauthenticated, "invalid token")
)

var peers []pb.WasabeeFederationClient

func Start(ctx context.Context) {
	c := config.Get()
	fc := path.Join(c.Certs, c.CertFile)
	key := path.Join(c.Certs, c.CertKey)

	cert, err := tls.LoadX509KeyPair(fc, key)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infow("startup", "message", "starting gRPC listener")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Get().GRPCPort))
	if err != nil {
		log.Error(err)
		return
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(ensureValidToken),
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
	)
	pb.RegisterWasabeeFederationServer(s, &wafed{})

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	defer s.GracefulStop()

	// start up client per-peer
	clientcreds, err := credentials.NewClientTLSFromFile(fc, "")
	if err != nil {
		log.Error(err)
		return
	}

	keypath := path.Join(c.Certs, c.FirebaseKey)
	perRPC, err := oauth.NewJWTAccessFromFile(keypath)
	if err != nil {
		log.Error(err)
		return
	}

	for _, p := range config.Get().Peers {
		conn, err := grpc.Dial(
			p,
			grpc.WithPerRPCCredentials(perRPC),
			grpc.WithTransportCredentials(clientcreds),
		)
		if err != nil {
			log.Info(err)
			// continue
		}
		// defer conn.Close()

		c := pb.NewWasabeeFederationClient(conn)
		peers = append(peers, c)
	}

	<-ctx.Done()
	log.Infow("shutdown", "message", "stopping gRPC listener")
}

func ensureValidToken(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errMissingMetadata
	}

	// log.Debugw("gRPC metadata", "md", md)

	if !valid(md["authorization"]) {
		return nil, errInvalidToken
	}
	return handler(ctx, req)
}

func valid(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}

	rawjwt := strings.TrimPrefix(authorization[0], "Bearer ")

	//		jwt.WithKeySet(keys),
	token, err := jwt.Parse([]byte(rawjwt),
		jwt.WithValidate(true),
		jwt.InferAlgorithmFromKey(true),
		jwt.UseDefaultKey(true),
		jwt.WithAcceptableSkew(20*time.Second))
	if err != nil {
		log.Error(err)
		return false
	}
	if token == nil {
		err := fmt.Errorf("unable to verify gRPC call")
		log.Error(err)
		return false
	}

	// m, _ := token.AsMap(context.TODO())
	// log.Debugw("token", "t", m)

	iss := token.Issuer()
	sub := token.Subject()
	dom := config.Get().GRPCDomain
	if !strings.Contains(iss, dom) || !strings.Contains(sub, dom) {
		log.Info("federation JWT creds", "iss", iss, "sub", sub, "dom", dom)
		err := fmt.Errorf("unable to validate gRPC caller")
		log.Error(err)
		return false
	}

	return true
}
