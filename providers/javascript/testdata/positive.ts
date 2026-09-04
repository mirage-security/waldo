export const schedule = () => {
	const expiryTimer = setTimeout(async () => {
		await deliverExpiry();
	}, 1_000);
	expiryTimer.unref();
};

declare const deliverExpiry: () => Promise<void>;
