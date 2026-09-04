const answered: Record<string, boolean> = {};

// One request records the state and a later request asks whether it happened.
export const markAnswered = (callID: string) => {
	answered[callID] = true;
};

export const hasAnswered = (callID: string) => {
	return answered[callID];
};
