// Package repositories contains MongoDB repository implementations.
package repositories

import "go.mongodb.org/mongo-driver/v2/bson"

// cmdSet - See https://www.mongodb.com/docs/manual/reference/operator/update/set/
func cmdSet(i any) bson.E {
	return bson.E{
		Key:   "$set",
		Value: i,
	}
}

// cmdUnset - See https://www.mongodb.com/docs/manual/reference/operator/update/unset/
func cmdUnset(i any) bson.E {
	return bson.E{
		Key:   "$unset",
		Value: i,
	}
}

// cmdInc - See https://www.mongodb.com/docs/manual/reference/operator/update/inc/
func cmdInc(i any) bson.E {
	return bson.E{
		Key:   "$inc",
		Value: i,
	}
}

// cmdPush - See https://www.mongodb.com/docs/manual/reference/operator/update/push/
func cmdPull(i any) bson.E {
	return bson.E{
		Key:   "$pull",
		Value: i,
	}
}

// cmdAddToSet - See https://www.mongodb.com/docs/manual/reference/operator/update/addToSet/
func cmdAddToSet(i any) bson.E {
	return bson.E{
		Key:   "$addToSet",
		Value: i,
	}
}

// cmdMax - See https://www.mongodb.com/docs/manual/reference/operator/update/max/
func cmdMax(i any) bson.E {
	return bson.E{
		Key:   "$max",
		Value: i,
	}
}

// cmdMatch - See https://www.mongodb.com/docs/manual/reference/operator/aggregation/match/
func cmdMatch(i any) bson.E {
	return bson.E{
		Key:   "$match",
		Value: i,
	}
}

// cmdGroup - See https://www.mongodb.com/docs/manual/reference/operator/aggregation/group/
func cmdGroup(i any) bson.E {
	return bson.E{
		Key:   "$group",
		Value: i,
	}
}

// cmdCount - See https://www.mongodb.com/docs/manual/reference/operator/aggregation/count/
func cmdCount(i any) bson.E {
	return bson.E{
		Key:   "$count",
		Value: i,
	}
}

// cmdProject - See https://www.mongodb.com/docs/manual/reference/operator/aggregation/project/
func cmdProject(i any) bson.E {
	return bson.E{
		Key:   "$project",
		Value: i,
	}
}

// cmdSort - See https://www.mongodb.com/docs/manual/reference/operator/aggregation/sort/
func cmdSort(i any) bson.E {
	return bson.E{
		Key:   "$sort",
		Value: i,
	}
}

// cmdLimit - See https://www.mongodb.com/docs/manual/reference/operator/aggregation/limit/
func cmdLimit(i any) bson.E {
	return bson.E{
		Key:   "$limit",
		Value: i,
	}
}
