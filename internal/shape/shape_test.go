package shape

import "testing"

func TestParseAppRef(t *testing.T) {
	t.Parallel()

	ref, err := ParseAppRef("crm/lead")
	if err != nil {
		t.Fatalf("ParseAppRef() error = %v, want nil", err)
	}
	if ref.App != "crm" || ref.Name != "lead" {
		t.Fatalf("ParseAppRef() = %+v, want crm/lead", ref)
	}
}

func TestParseAppRefRejectsInvalidTargets(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"lead", "crm/", "/lead", "crm/lead/extra", "CRM/lead", "crm/Lead"} {
		if _, err := ParseAppRef(target); err == nil {
			t.Fatalf("ParseAppRef(%q) error = nil, want error", target)
		}
	}
}

func TestCanonicalPaths(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		AppDir("crm"):                       "apps/crm",
		AppManifestPath("crm"):              "apps/crm/app.yml",
		AppEntitiesPath("crm"):              "apps/crm/entities",
		AppCollectionDirPath("crm"):         "apps/crm/entities/_collections",
		AppJobsPath("crm"):                  "apps/crm/jobs",
		AppSchedulesPath("crm"):             "apps/crm/jobs/_schedules.yml",
		AppPagesPath("crm"):                 "apps/crm/pages",
		AppReportsPath("crm"):               "apps/crm/reports",
		AppRolesPath("crm"):                 "apps/crm/roles.yml",
		EntityDir("lead"):                   "entities/lead",
		EntityMetadataPath("lead"):          "entities/lead/entity.yml",
		EntityFixturesPath("lead"):          "entities/lead/fixtures.yml",
		EntityHooksPath("lead"):             "entities/lead/hooks.go",
		EntityPermissionsPath("lead"):       "entities/lead/permissions.yml",
		EntityViewsPath("lead"):             "entities/lead/views.yml",
		CollectionMetadataPath("row"):       "entities/_collections/row.yml",
		CollectionBundleMetadataPath("row"): "entities/_collections/row/entity.yml",
		JobMetadataPath("send-email"):       "jobs/send-email/job.yml",
		JobRunPath("send-email"):            "jobs/send-email/run.go",
		PageMetadataPath("dashboard"):       "pages/dashboard/page.yml",
		ReportFilePath("pipeline"):          "reports/pipeline.yml",
		ReportMetadataPath("pipeline"):      "reports/pipeline/report.yml",
		LocalStudioAppDir:                   ".dygo/apps/studio",
		LocalFilesDir:                       ".dygo/files",
		LocalLogsDir:                        ".dygo/logs",
		LocalTempDir:                        ".dygo/tmp",
		LocalSecretsDir:                     ".dygo/secrets",
		LocalSecretKeyFile:                  ".dygo/secrets/master.key",
		LocalSecretsTempDir:                 ".dygo/secrets/tmp",
	}
	for got, want := range tests {
		if got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
	}
}
