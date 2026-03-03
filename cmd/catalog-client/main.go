package main

import (
	"context"
	"flag"
	"log"
	"time"

	catalogv1 "github.com/YagorX/shop-contracts/gen/go/proto/catalog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9091", "grpc server address")
	id := flag.String("id", "prod-001", "product id")
	flag.Parse()

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := catalogv1.NewCatalogServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	listResp, err := client.ListProducts(ctx, &catalogv1.ListProductsRequest{
		Limit:  3,
		Offset: 0,
	})
	if err != nil {
		log.Fatalf("ListProducts: %v", err)
	}
	log.Printf("ListProducts total=%d items=%d", listResp.GetTotal(), len(listResp.GetItems()))

	getResp, err := client.GetProduct(ctx, &catalogv1.GetProductRequest{
		Id: *id,
	})
	if err != nil {
		log.Fatalf("GetProduct: %v", err)
	}
	log.Printf("GetProduct id=%s name=%s price=%.2f", getResp.GetProduct().GetId(), getResp.GetProduct().GetName(), getResp.GetProduct().GetPrice())
}
