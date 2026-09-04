export const schedule = () => {
	const cleanupTimer = setTimeout(() => {
		cleanupScratchState();
	}, 1_000);
	cleanupTimer.unref();
};

declare const cleanupScratchState: () => void;
