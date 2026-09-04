export const hasDuplicates = (values: string[]) => {
	const seen: Record<string, boolean> = {};
	for (const value of values) {
		if (seen[value]) return true;
		seen[value] = true;
	}
	return false;
};
