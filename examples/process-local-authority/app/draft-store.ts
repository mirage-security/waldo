interface Draft {
	body: string;
}

const drafts = new Map<string, Draft>();

export function saveDraft(id: string, body: string): void {
	drafts.set(id, { body });
}

export function loadDraft(id: string): Draft | null {
	return drafts.get(id) ?? null;
}
