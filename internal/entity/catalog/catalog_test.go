package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

func TestValidateLoadsEntitiesFromManifestPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{Entities: "metadata/entities"})
	entityPath := filepath.Join(app.Dir, "metadata", "entities", "lead", "lead.entity.yml")
	writeEntity(t, entityPath, "lead")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(entities) != 1 {
		t.Fatalf("Validate() len = %d, want 1", len(entities))
	}

	entity := entities[0]
	if entity.AppName != "sales" {
		t.Fatalf("LoadedEntity.AppName = %q, want sales", entity.AppName)
	}
	if entity.AppDir != app.Dir {
		t.Fatalf("LoadedEntity.AppDir = %q, want %q", entity.AppDir, app.Dir)
	}
	if entity.Path != entityPath {
		t.Fatalf("LoadedEntity.Path = %q, want %q", entity.Path, entityPath)
	}
	if entity.Entity.Name != "lead" {
		t.Fatalf("LoadedEntity.Entity.Name = %q, want lead", entity.Entity.Name)
	}
}

func TestValidateAllowsMissingEntitiesDirectory(t *testing.T) {
	t.Parallel()

	app := loadedApp(t.TempDir(), "sales", "sales", manifest.Paths{})

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(entities) != 0 {
		t.Fatalf("Validate() len = %d, want 0", len(entities))
	}
}

func TestValidateReturnsDeterministicOrdering(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{})
	core := loadedApp(root, "core", "core", manifest.Paths{})
	writeEntity(t, entityPath(sales, "lead"), "lead")
	writeEntity(t, entityPath(sales, "company"), "company")
	writeEntity(t, entityPath(core, "user"), "user")

	entities, err := New([]manifest.LoadedApp{sales, core}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	got := entityKeys(entities)
	want := "core/user,sales/company,sales/lead"
	if strings.Join(got, ",") != want {
		t.Fatalf("Validate() order = %q, want %q", strings.Join(got, ","), want)
	}
}

func TestValidateRejectsInvalidEntityWithAppAndPathContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	badPath := entityPath(app, "bad")
	writeFile(t, badPath, `
fields:
  - name: title
    label: Title
    type: text
`)

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid entity error")
	}
	for _, want := range []string{`app "sales"`, badPath, "label is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateLoadsCanonicalEntityBundle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	entityPath := entityPath(app, "lead")
	writeEntity(t, entityPath, "lead")
	writeFile(t, filepath.Join(app.Dir, "entities", "lead", "fixtures.yml"), "records: []")
	writeFile(t, filepath.Join(app.Dir, "entities", "lead", "views.yml"), "views: []")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/lead" {
		t.Fatalf("Validate() entities = %#v, want canonical lead bundle", got)
	}
	if entities[0].Path != entityPath {
		t.Fatalf("LoadedEntity.Path = %q, want %q", entities[0].Path, entityPath)
	}
}

func TestValidateRejectsInvalidEntityBundleFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, filepath.Join(app.Dir, "entities", "lead", "lead.yml"), "lead")

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid bundle file error")
	}
	if !strings.Contains(err.Error(), `entities/lead/lead.yml is not a valid Entity bundle file; Entity metadata must be entities/lead/lead.entity.yml`) {
		t.Fatalf("Validate() error = %q, want invalid bundle file context", err.Error())
	}
}

func TestValidateRejectsBundleFilesWithoutEntityMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, filepath.Join(app.Dir, "entities", "lead", "fixtures.yml"), "records: []")

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing entity metadata error")
	}
	if !strings.Contains(err.Error(), `entities/lead requires Entity metadata file entities/lead/lead.entity.yml`) {
		t.Fatalf("Validate() error = %q, want missing entity metadata context", err.Error())
	}
}

func TestValidateAllowsDuplicateEntityNamesAcrossAppsWithUniqueRouteSlugs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{})
	support := loadedApp(root, "support", "support", manifest.Paths{})
	writeEntity(t, entityPath(sales, "customer"), "customer")
	writeEntityWithRoute(t, entityPath(support, "customer"), "customer", "support-customer")

	entities, err := New([]manifest.LoadedApp{sales, support}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/customer,support/customer" {
		t.Fatalf("Validate() entities = %#v, want duplicate names across apps", got)
	}
}

func TestValidateRejectsDuplicateRouteSlugsAcrossApps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{})
	support := loadedApp(root, "support", "support", manifest.Paths{})
	writeEntity(t, entityPath(sales, "customer"), "customer")
	supportPath := entityPath(support, "customer")
	writeEntity(t, supportPath, "customer")

	_, err := New([]manifest.LoadedApp{sales, support}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate route slug error")
	}
	for _, want := range []string{supportPath + ":1", `app "support"`, `entity "customer"`, `route slug "customer" conflicts`, `set route.slug`, `support-customer`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateAllowsCurrentNonReservedRootSlugs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	for _, name := range []string{"settings", "spaces", "pages", "reports", "new"} {
		writeEntity(t, entityPath(app, name), name)
	}

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(entities) != 5 {
		t.Fatalf("Validate() len = %d, want 5", len(entities))
	}
}

func TestValidateRejectsReservedRootRouteSlugs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		entityName string
		routeSlug  string
		wantSlug   string
	}{
		{name: "default route slug", entityName: "login", wantSlug: "login"},
		{name: "explicit route slug", entityName: "lead", routeSlug: "health", wantSlug: "health"},
		{name: "new reserved route slug", entityName: "settings", routeSlug: "setup", wantSlug: "setup"},
		{name: "me route slug", entityName: "me", wantSlug: "me"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			app := loadedApp(root, "sales", "sales", manifest.Paths{})
			entityPath := entityPath(app, tt.entityName)
			if tt.routeSlug == "" {
				writeEntity(t, entityPath, tt.entityName)
			} else {
				writeEntityWithRoute(t, entityPath, tt.entityName, tt.routeSlug)
			}

			_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want reserved slug error")
			}
			for _, want := range []string{entityPath + ":1", `app "sales"`, `entity "` + tt.entityName + `"`, `reserved root route slug "` + tt.wantSlug + `"`, `set route.slug`} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
				}
			}
		})
	}
}

func TestValidateAcceptsResolvedFieldTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "company"), "company")
	writeFile(t, entityPath(app, "lead"), `
label: Lead
name:
  strategy: random
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
  - name: contacts
    label: Contacts
    type: collection
    options:
      entity: lead-contact
`)
	writeCollectionEntity(t, collectionPath(app, "lead-contact"), "lead-contact")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(entities) != 3 {
		t.Fatalf("Validate() len = %d, want 3", len(entities))
	}
	for _, entity := range entities {
		if entity.Entity.Name == "lead-contact" {
			if !entity.IsCollection() || !entity.Entity.IsCollection {
				t.Fatalf("lead-contact collection flags = IsCollection %v entity flag %v, want collection", entity.IsCollection(), entity.Entity.IsCollection)
			}
			return
		}
	}
	t.Fatal("Validate() did not load lead-contact collection Entity")
}

func TestValidateRejectsSingleEntityFieldTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "invoice-settings"), `
label: Invoice Settings
is-single: true
fields:
  - name: default-due-days
    label: Default Due Days
    type: int
    required: true
    default: 30
`)
	entityPath := entityPath(app, "invoice")
	writeFile(t, entityPath, `
label: Invoice
name:
  strategy: random
fields:
  - name: settings
    label: Settings
    type: link
    options:
      entity: invoice-settings
`)

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want single target error")
	}
	for _, want := range []string{entityPath + ":5", `field "settings"`, `single Entity "invoice-settings"`, `cannot be link targets`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateAllowsSingleEntityToLinkToNormalEntity(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "customer"), "customer")
	writeFile(t, entityPath(app, "invoice-settings"), `
label: Invoice Settings
is-single: true
fields:
  - name: default-customer
    label: Default Customer
    type: link
    options:
      entity: customer
`)

	if _, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateAcceptsFieldFetchPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "customer"), `
label: Customer
name:
  strategy: random
fields:
  - name: title
    label: Title
    type: text
`)
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: customer
    label: Customer
    type: link
    options:
      entity: customer
  - name: customer-title
    label: Customer Title
    type: text
    fetch:
      from: customer.title
`)

	if _, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateAcceptsFieldFetchLinkToLinkPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "currency"), "currency")
	writeFile(t, entityPath(app, "customer"), `
label: Customer
name:
  strategy: random
fields:
  - name: default-currency
    label: Default Currency
    type: link
    options:
      entity: currency
`)
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: customer
    label: Customer
    type: link
    options:
      entity: customer
  - name: currency
    label: Currency
    type: link
    fetch:
      from: customer.default-currency
    options:
      entity: currency
`)

	if _, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsInvalidFieldFetchPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "customer"), `
label: Customer
name:
  strategy: random
fields:
  - name: title
    label: Title
    type: text
`)
	invoicePath := entityPath(app, "invoice")
	writeFile(t, invoicePath, `
label: Invoice
name:
  strategy: random
fields:
  - name: customer
    label: Customer
    type: link
    options:
      entity: customer
  - name: bad-title
    label: Bad Title
    type: text
    fetch:
      from: customer.missing
  - name: bad-link
    label: Bad Link
    type: text
    fetch:
      from: bad-title.title
`)

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want fetch errors")
	}
	for _, want := range []string{invoicePath, `field "bad-title"`, `unknown field "missing"`, `field "bad-link"`, `must be a link field`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateRejectsMissingFieldTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	entityPath := entityPath(app, "lead")
	writeFile(t, entityPath, `
label: Lead
name:
  strategy: random
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
`)

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing target error")
	}
	for _, want := range []string{entityPath + ":5", `app "sales"`, `entity "lead"`, `field "company"`, `unknown entity target "company"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateResolvesSameAppFieldTargetBeforeGlobalNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{})
	support := loadedApp(root, "support", "support", manifest.Paths{})
	writeEntity(t, entityPath(sales, "customer"), "customer")
	writeEntityWithRoute(t, entityPath(support, "customer"), "customer", "support-customer")
	entityPath := entityPath(sales, "lead")
	writeFile(t, entityPath, `
label: Lead
name:
  strategy: random
fields:
  - name: customer
    label: Customer
    type: link
    options:
      entity: customer
`)

	_, err := New([]manifest.LoadedApp{sales, support}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsAmbiguousFieldTargetAcrossApps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{})
	support := loadedApp(root, "support", "support", manifest.Paths{})
	billing := loadedApp(root, "billing", "billing", manifest.Paths{})
	writeEntityWithRoute(t, entityPath(support, "customer"), "customer", "support-customer")
	writeEntityWithRoute(t, entityPath(billing, "customer"), "customer", "billing-customer")
	entityPath := entityPath(sales, "lead")
	writeFile(t, entityPath, `
label: Lead
name:
  strategy: random
fields:
  - name: customer
    label: Customer
    type: link
    options:
      entity: customer
`)

	_, err := New([]manifest.LoadedApp{sales, support, billing}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want ambiguous target error")
	}
	for _, want := range []string{entityPath + ":5", `app "sales"`, `field "customer"`, `ambiguous entity target "customer"`, `set options.app`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateLoadsCollectionEntitiesFromCollectionsFolder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
`)
	writeCollectionEntity(t, collectionPath(app, "invoice-item"), "invoice-item")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/invoice,sales/invoice-item" {
		t.Fatalf("Validate() entities = %#v, want invoice parent and invoice-item collection", got)
	}
	if entities[0].IsCollection() {
		t.Fatal("invoice IsCollection = true, want false")
	}
	if !entities[1].IsCollection() {
		t.Fatalf("invoice-item IsCollection = %v, want collection", entities[1].IsCollection())
	}
	if entities[1].RouteSlug() != "" || entities[1].HasRouteSlug() {
		t.Fatalf("invoice-item route slug = %q routeable = %v, want no public route slug", entities[1].RouteSlug(), entities[1].HasRouteSlug())
	}
}

func TestValidateLoadsCanonicalCollectionEntityFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
`)
	writeCollectionEntity(t, filepath.Join(app.Dir, "entities", "_collections", "invoice-item.yml"), "invoice-item")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/invoice,sales/invoice-item" {
		t.Fatalf("Validate() entities = %#v, want canonical invoice and collection", got)
	}
	if !entities[1].IsCollection() {
		t.Fatalf("invoice-item IsCollection = %v, want collection", entities[1].IsCollection())
	}
}

func TestValidateLoadsCanonicalCollectionEntityBundles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
`)
	entityPath := filepath.Join(app.Dir, "entities", "_collections", "invoice-item", "invoice-item.entity.yml")
	writeCollectionEntity(t, entityPath, "invoice-item")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/invoice,sales/invoice-item" {
		t.Fatalf("Validate() entities = %#v, want canonical invoice and collection bundle", got)
	}
	if entities[1].Path != entityPath {
		t.Fatalf("LoadedEntity.Path = %q, want %q", entities[1].Path, entityPath)
	}
	if !entities[1].IsCollection() {
		t.Fatalf("invoice-item IsCollection = %v, want collection", entities[1].IsCollection())
	}
}

func TestValidateRejectsCollectionEntityRouteSlug(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
`)
	writeFile(t, collectionPath(app, "invoice-item"), `
label: Invoice Item
route:
  slug: invoice-item
fields:
  - name: title
    label: Title
    type: text
`)

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want collection route slug error")
	}
	if !strings.Contains(err.Error(), `collection Entity "invoice-item" cannot define route.slug`) {
		t.Fatalf("Validate() error = %q, want collection route slug context", err.Error())
	}
}

func TestValidateIgnoresCollectionEntitiesForRouteSlugUniqueness(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	support := loadedApp(root, "support", "support", manifest.Paths{})
	writeEntity(t, entityPath(support, "customer"), "customer")
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: rows
    label: Rows
    type: collection
    options:
      entity: customer
`)
	writeCollectionEntity(t, collectionPath(app, "customer"), "customer")

	entities, err := New([]manifest.LoadedApp{app, support}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/customer,sales/invoice,support/customer" {
		t.Fatalf("Validate() entities = %#v, want normal and collection customer identities", got)
	}
}

func TestValidateAllowsUnusedCollectionEntity(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "invoice"), "invoice")
	writeCollectionEntity(t, collectionPath(app, "invoice-item"), "invoice-item")

	if _, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateAllowsCollectionEntityReferencedMoreThanOnce(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
  - name: extra-items
    label: Extra Items
    type: collection
    options:
      entity: invoice-item
`)
	writeCollectionEntity(t, collectionPath(app, "invoice-item"), "invoice-item")

	if _, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsLinksToCollectionEntities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
  - name: featured-item
    label: Featured Item
    type: link
    options:
      entity: invoice-item
`)
	writeCollectionEntity(t, collectionPath(app, "invoice-item"), "invoice-item")

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want collection link target error")
	}
	if !strings.Contains(err.Error(), `links to collection Entity "invoice-item"; collection Entities cannot be link targets`) {
		t.Fatalf("Validate() error = %q, want collection link target context", err.Error())
	}
}

func TestValidateAllowsCrossAppCollectionTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{})
	crm := loadedApp(root, "crm", "crm", manifest.Paths{})
	writeCollectionEntity(t, collectionPath(crm, "contact-row"), "contact-row")
	writeFile(t, entityPath(sales, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: contacts
    label: Contacts
    type: collection
    options:
      app: crm
      entity: contact-row
`)

	if _, err := New([]manifest.LoadedApp{sales, crm}, fieldtype.DefaultRegistry()).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsCollectionFieldsTargetingNormalEntities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "invoice-item"), "invoice-item")
	writeFile(t, entityPath(app, "invoice"), `
label: Invoice
name:
  strategy: random
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
`)

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want normal target error")
	}
	if !strings.Contains(err.Error(), `targets Entity "invoice-item", but collection fields must target a collection Entity`) {
		t.Fatalf("Validate() error = %q, want normal collection target context", err.Error())
	}
}

func TestValidateRejectsDuplicateNormalAndCollectionEntityNamesInSameApp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "invoice"), "invoice")
	writeCollectionEntity(t, collectionPath(app, "invoice"), "invoice")

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate entity identity error")
	}
	if !strings.Contains(err.Error(), `app "sales" entity "invoice" duplicates Entity identity`) {
		t.Fatalf("Validate() error = %q, want duplicate normal/collection context", err.Error())
	}
}

func TestDiscoverIgnoresNonYAMLFilesAndNestedDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "lead"), "lead")
	writeFile(t, filepath.Join(app.Dir, "entities", "ignored.yaml"), "not: valid: yaml")
	writeFile(t, filepath.Join(app.Dir, "entities", "notes.txt"), "not an entity")
	writeEntity(t, filepath.Join(app.Dir, "entities", "nested", "deeper", "bad.yml"), "bad")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/lead" {
		t.Fatalf("Validate() entities = %#v, want [sales/lead]", got)
	}
}

func TestValidateAcceptsHookFilesMatchingEntityNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{})
	writeEntity(t, entityPath(app, "lead"), "lead")
	writeFile(t, filepath.Join(app.Dir, "entities", "lead", "hooks.go"), "package hooks")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/lead" {
		t.Fatalf("Validate() entities = %#v, want [sales/lead]", got)
	}
}

func loadedApp(root string, dirName string, name string, paths manifest.Paths) manifest.LoadedApp {
	dir := filepath.Join(root, dirName)
	return manifest.LoadedApp{
		Dir:          dir,
		ManifestPath: filepath.Join(dir, manifest.Filename),
		Manifest: manifest.Manifest{
			Name:    name,
			Label:   labelForName(name),
			Version: "0.1.0",
			Paths:   paths.WithDefaults(),
		},
	}
}

func writeEntity(t *testing.T, path string, name string) {
	t.Helper()

	writeFile(t, path, `
label: `+labelForName(name)+`
name:
  strategy: random
fields:
  - name: title
    label: Title
    type: text
`)
}

func entityPath(app manifest.LoadedApp, name string) string {
	return filepath.Join(app.Dir, "entities", name, name+".entity.yml")
}

func collectionPath(app manifest.LoadedApp, name string) string {
	return filepath.Join(app.Dir, "entities", "_collections", name+".yml")
}

func writeCollectionEntity(t *testing.T, path string, name string) {
	t.Helper()

	writeFile(t, path, `
label: `+labelForName(name)+`
fields:
  - name: title
    label: Title
    type: text
`)
}

func writeEntityWithRoute(t *testing.T, path string, name string, routeSlug string) {
	t.Helper()

	writeFile(t, path, `
label: `+labelForName(name)+`
name:
  strategy: random
route:
  slug: `+routeSlug+`
fields:
  - name: title
    label: Title
    type: text
`)
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func entityKeys(entities []LoadedEntity) []string {
	keys := make([]string, 0, len(entities))
	for _, entity := range entities {
		keys = append(keys, entity.AppName+"/"+entity.Entity.Name)
	}
	return keys
}

func labelForName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
