package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"time"

	"go.uber.org/zap"

	// "golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	// "google.golang.org/grpc/credentials/oauth"

	"github.com/wasabee-project/Wasabee-Server/config"
	wrpc "github.com/wasabee-project/Wasabee-Server/federation/pb"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

var addr = flag.String("addr", "localhost:51500", "the address to connect to")

func callRPC(client wrpc.WasabeeFederationClient, gid model.GoogleID, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.SetCommunityID(ctx, &wrpc.CommunityID{
		Googleid:      string(gid),
		Communityname: name,
	})
	if err != nil {
		log.Error(err)
	}
	fmt.Println("response: ", resp.Message)
}

func main() {
	flag.Parse()

	logconf := log.Configuration{
		Console:      true,
		ConsoleLevel: zap.DebugLevel,
		// FilePath:           cargs.String("log"),
		FileLevel: zap.InfoLevel,
	}
	log.Start(context.Background(), &logconf)

	c, err := config.LoadFile("wasabee.json")
	if err != nil {
		log.Fatal(err)
	}
	fc := path.Join(c.Certs, c.CertFile)

	clientcreds, err := credentials.NewClientTLSFromFile(fc, "scar.indievisible.org")
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

	opts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(perRPC),
		grpc.WithTransportCredentials(clientcreds),
	}

	conn, err := grpc.Dial(*addr, opts...)
	if err != nil {
		log.Error(err)
	}
	defer conn.Close()
	rc := wrpc.NewWasabeeFederationClient(conn)

	callRPC(rc, model.GoogleID("112233445500"), "test")
}
