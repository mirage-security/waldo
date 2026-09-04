export const schedule = () => {
	const testTimer = setTimeout(async () => {
		await flushFixture();
	}, 1_000);
	testTimer.unref();
};

declare const flushFixture: () => Promise<void>;
