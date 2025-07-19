package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Baito struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Title        string             `bson:"title" json:"title"`
	Description  string             `bson:"description" json:"description"`
	Category     string             `bson:"category" json:"category"`
	SubCategory  string             `bson:"subcategory" json:"subcategory"`
	Location     string             `bson:"location" json:"location"`
	Wage         string             `bson:"wage" json:"wage"`
	Phone        string             `bson:"phone" json:"phone"`
	Requirements string             `bson:"requirements" json:"requirements"`
	BannerURL    string             `bson:"banner,omitempty" json:"banner,omitempty"`
	Images       []string           `bson:"images" json:"images"`
	WorkHours    string             `bson:"workHours" json:"workHours"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	OwnerID      string             `bson:"ownerId" json:"ownerId"`
}

type BaitoApplication struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	BaitoID     primitive.ObjectID `bson:"baitoId" json:"baitoId"`
	UserID      string             `bson:"userid" json:"userid"`
	Username    string             `bson:"username" json:"username"`
	Pitch       string             `bson:"pitch" json:"pitch"`
	SubmittedAt time.Time          `bson:"submittedAt" json:"submittedAt"`
}
