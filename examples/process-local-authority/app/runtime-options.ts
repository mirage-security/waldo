interface RuntimeOption {
	value: string;
}

const options: Record<string, RuntimeOption> = {};

const writeOption = (name: string, option: RuntimeOption): void => {
	options[name] = option;
};

const readOption = (name: string): RuntimeOption | null => {
	if (options[name]) {
		return options[name];
	}
	return null;
};

export function configureOption(name: string, value: string): void {
	writeOption(name, { value });
}

export function optionValue(name: string): string | null {
	return readOption(name)?.value ?? null;
}
