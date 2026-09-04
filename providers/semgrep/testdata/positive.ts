const active: Record<string, boolean> = {};

export const markActive = (key: string) => {
	active[key] = true;
};

export const hasActive = (key: string) => {
	return active[key];
};
