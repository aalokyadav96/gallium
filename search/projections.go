package search

import (
	"go.mongodb.org/mongo-driver/bson"
)

var Projections = map[string]bson.M{
	"artists": {
		"artistid": 1, "category": 1, "name": 1, "country": 1,
		"bio": 1, "dob": 1, "photo": 1, "banner": 1,
		"genres": 1, "events": 1, "members": 1,
	},
	"baitos": {
		"baitoid": 1, "title": 1, "description": 1,
		"category": 1, "subcategory": 1, "location": 1, "wage": 1,
		"requirements": 1, "banner": 1, "workHours": 1,
		"createdAt": 1, "ownerId": 1,
	},
	"baitoworkers": {
		"baito_user_id": 1, "name": 1, "age": 1, "preferred_roles": 1,
		"bio": 1, "avatar": 1, "created_at": 1,
	},
	"blogposts": {
		"postid": 1, "title": 1, "content": 1,
		"category": 1, "subcategory": 1, "imagePaths": 1,
		"referenceId": 1, "createdBy": 1, "createdAt": 1, "updatedAt": 1,
	},
	"crops": {
		"cropid": 1, "name": 1, "price": 1, "quantity": 1, "unit": 1,
		"imageUrl": 1, "category": 1, "harvestDate": 1,
		"expiryDate": 1, "updatedAt": 1, "createdAt": 1, "farmId": 1,
	},
	"events": {
		"eventid": 1, "title": 1, "location": 1, "date": 1,
		"category": 1, "description": 1, "prices": 1, "banner_image": 1,
	},
	"media": {
		"mediaid": 1, "entityid": 1, "entitytype": 1,
		"media": 1, "createdAt": 1,
	},
	"menu": {
		"menuid": 1, "placeid": 1, "name": 1, "description": 1,
	},
	"merch": {
		"merchid": 1, "name": 1, "price": 1,
		"description": 1, "merch_pic": 1,
	},
	"places": {
		"placeid": 1, "name": 1, "location": 1,
		"category": 1, "banner": 1, "description": 1,
	},
	"feedposts": {
		"postid": 1, "userid": 1, "type": 1, "content": 1, "timestamp": 1, "media": 1,
	},
	"products": {
		"name": 1, "productid": 1, "description": 1, "price": 1,
		"imageUrls": 1, "category": 1, "type": 1,
		"quantity": 1, "unit": 1, "availableFrom": 1, "availableTo": 1,
	},
	"recipes": {
		"recipeid": 1, "userId": 1, "title": 1, "description": 1,
		"prepTime": 1, "tags": 1, "imageUrls": 1, "ingredients": 1,
		"difficulty": 1, "servings": 1, "createdAt": 1,
	},
	"songs": {
		"songid": 1, "title": 1, "description": 1,
		"uploadedAt": 1, "artistid": 1, "genre": 1,
	},
	"users": {
		"userid": 1, "username": 1, "name": 1, "bio": 1, "avatar": 1,
	},
	"farms": {
		"farmid": 1, "name": 1, "latitude": 1, "longitude": 1,
		"description": 1, "owner": 1, "availabilityTiming": 1,
		"tags": 1, "photo": 1, "crops": 1, "avgRating": 1,
		"reviewCount": 1, "favoritesCount": 1, "createdBy": 1,
		"createdAt": 1, "updatedAt": 1,
	},
}
