interface CatalogEntry {
	label: string;
}

const catalog = new Map<string, CatalogEntry>();

export function cacheCatalogEntry(id: string, entry: CatalogEntry): void {
	catalog.set(id, entry);
}

export function findCachedCatalogEntry(id: string): CatalogEntry | null {
	return catalog.get(id) ?? null;
}
