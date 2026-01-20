function geohashRangeSingle(parent: string) {
	if (!parent || parent.length === 0) {
		throw new Error('Parent geohash cannot be empty');
	}

	const lastChar = parent[parent.length - 1];
	const prefix = parent.slice(0, -1);

	// Check if last char is 'z' or 'Z', in which case we omit $lt
	if (lastChar.toLowerCase() === 'z') {
		return { 'position.geohash': { $gte: parent } };
	}

	// Increment last character
	const nextChar = String.fromCharCode(lastChar.charCodeAt(0) + 1);

	return {
		'position.geohash': { $gte: parent, $lt: prefix + nextChar },
	};
}

export function geohashRange(parent: string[]) {
	if (!parent || (Array.isArray(parent) && parent.length === 0)) {
		throw new Error('Parent geohash cannot be empty');
	}

	// Handle array of strings
	if (parent.length === 1) {
		return geohashRangeSingle(parent[0]);
	}

	// Multiple geohashes - return $or query
	return {
		$or: parent.map(p => geohashRangeSingle(p)),
	};
}
