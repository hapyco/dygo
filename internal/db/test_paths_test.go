package db

func testEntityPath(appName string, entityName string) string {
	return "apps/" + appName + "/entities/" + entityName + "/" + entityName + ".entity.yml"
}

func testCollectionEntityPath(appName string, entityName string) string {
	return "apps/" + appName + "/entities/_collections/" + entityName + "/" + entityName + ".entity.yml"
}
