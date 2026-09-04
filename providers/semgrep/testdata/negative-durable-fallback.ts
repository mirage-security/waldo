const cache: Record<string, boolean> = {};

export const remember = (key: string) => {
	cache[key] = true;
};

export const hasDurableRecord = async (key: string) => {
	return cache[key] ?? (await readDurableRecord(key));
};

declare function readDurableRecord(key: string): Promise<boolean>;
