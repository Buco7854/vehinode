DEFAULT_TELEMETRY_POLICY: dict[str, int] = {
    "sampling_seconds": 120,
    "upload_seconds": 120,
    "parked_sampling_seconds": 900,
    "parked_upload_seconds": 900,
}


def default_telemetry_policy() -> dict[str, int]:
    return dict(DEFAULT_TELEMETRY_POLICY)
