package products

import (
	"context"
	"naevis/db"
	"naevis/models"

	"go.mongodb.org/mongo-driver/bson"
)

func getProductEntity(ctx context.Context, id string) models.Product {

	var product models.Product
	if err := db.ProductCollection.FindOne(ctx, bson.M{"productid": id}).Decode(&product); err != nil {
		return product
	}
	return product
}
