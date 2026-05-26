package db

func testEntityPath(appName string, entityName string) string {
	return "apps/" + appName + "/entities/" + entityName + "/entity.yml"
}

func testCollectionEntityPath(appName string, entityName string) string {
	return "apps/" + appName + "/entities/_collections/" + entityName + "/entity.yml"
}
