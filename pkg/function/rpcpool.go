package function

import (
	"context"
	"io"
	"time"

	tc "github.com/eth-easl/loader/pkg/trace"
	grpcpool "github.com/processout/grpc-go-pool"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var pools = RpcPools{}

type RpcPools struct {
	pools     map[string]*grpcpool.Pool
	conns     map[string]*grpc.ClientConn
	contexts  map[string]context.Context
	callbacks map[string]context.CancelFunc
}

func (ps *RpcPools) GetConn(endpoint string) (*grpcpool.ClientConn, error) {
	pool := ps.pools[endpoint]
	return pool.Get(pools.contexts[endpoint])
}

func CreateGrpcPool(functions []tc.Function) {
	pools.pools = map[string]*grpcpool.Pool{}
	pools.conns = map[string]*grpc.ClientConn{}
	pools.contexts = map[string]context.Context{}
	pools.callbacks = map[string]context.CancelFunc{}

	for _, function := range functions {
		dailCxt, cancelDailing := context.WithTimeout(context.Background(), connectionTimeout)
		var factory grpcpool.Factory = func() (*grpc.ClientConn, error) {
			// defer cancelDailing()
			conn, err := grpc.DialContext(dailCxt, function.Endpoint+port, grpc.WithInsecure())
			if err != nil {
				log.Fatalf("Failed to start gRPC connection (%s): %v", function.Name, err)
			}
			log.Infof("New connection to function at %s", function.Endpoint)

			pools.conns[function.Endpoint] = conn
			return conn, err
		}
		pool, err := grpcpool.New(factory, 1, 1, time.Hour*2)
		if err != nil {
			log.Fatalf("Failed to create gRPC pool (%s): %v", function.Name, err)
		}

		pools.pools[function.Endpoint] = pool
		pools.contexts[function.Endpoint] = dailCxt
		pools.callbacks[function.Endpoint] = cancelDailing
	}
}

func DestroyGrpcPool() {
	for endpoint := range pools.pools {
		pools.callbacks[endpoint]()
		closeConn(pools.conns[endpoint])
	}
}

func closeConn(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Warn("Connection closing error: ", err)
	}
}