#!/usr/bin/env python3
"""Download one S3-compatible backup object without printing credentials."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import hmac
import json
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path


SENSITIVE_KEYS = {"secretkey", "s3_secret_access_key"}


def parse_scalar(value: str):
    value = value.strip().strip(",")
    if value.startswith("`") and value.endswith("`"):
        value = value[1:-1]
    if value.startswith('"') and value.endswith('"'):
        return value[1:-1]
    lower = value.lower()
    if lower in {"true", "false"}:
        return lower == "true"
    if lower in {"null", "none"}:
        return ""
    return value


def parse_credentials(path: Path) -> dict[str, object]:
    text = path.read_text(encoding="utf-8")
    data: dict[str, object] = {}
    for line in text.splitlines():
        match = re.match(r"^\s*[-*]?\s*`?([A-Za-z0-9_ .-]+)`?\s*[:=]\s*(.+?)\s*$", line)
        if match:
            key = match.group(1).strip().replace(" ", "")
            data[key] = parse_scalar(match.group(2))

    json_match = re.search(r"```json\s*(\{.*?\})\s*```", text, re.S)
    if json_match:
        try:
            target = json.loads(json_match.group(1))
            for key, value in target.items():
                data[key] = value
        except json.JSONDecodeError as exc:
            raise SystemExit(f"invalid JSON block in credentials file: {exc}") from exc
    return data


def pick(data: dict[str, object], *keys: str, required: bool = True) -> str:
    for key in keys:
        value = data.get(key)
        if value is None:
            continue
        text = str(value).strip()
        if text.startswith("<") and text.endswith(">"):
            continue
        if text:
            return text
    if required:
        raise SystemExit(f"missing required credential field: {'/'.join(keys)}")
    return ""


def bool_pick(data: dict[str, object], *keys: str) -> bool:
    for key in keys:
        value = data.get(key)
        if isinstance(value, bool):
            return value
        if isinstance(value, str):
            return value.strip().lower() in {"1", "true", "yes", "y"}
    return False


def uri_escape_path(path: str) -> str:
    return "/" + "/".join(urllib.parse.quote(part, safe="~") for part in path.strip("/").split("/") if part)


def object_url(endpoint: str, bucket: str, object_name: str, path_style: bool) -> str:
    parsed = urllib.parse.urlparse(endpoint)
    if not parsed.scheme or not parsed.netloc:
        raise SystemExit("s3 endpoint must be a valid URL")
    if path_style:
        path = "/".join(part.strip("/") for part in [parsed.path, bucket, object_name] if part.strip("/"))
        return urllib.parse.urlunparse(parsed._replace(path=uri_escape_path(path), params="", query="", fragment=""))
    host = f"{bucket}.{parsed.netloc}"
    path = "/".join(part.strip("/") for part in [parsed.path, object_name] if part.strip("/"))
    return urllib.parse.urlunparse(parsed._replace(netloc=host, path=uri_escape_path(path), params="", query="", fragment=""))


def hmac_sha256(key: bytes, value: str) -> bytes:
    return hmac.new(key, value.encode("utf-8"), hashlib.sha256).digest()


def signing_key(secret_key: str, short_date: str, region: str) -> bytes:
    date_key = hmac_sha256(("AWS4" + secret_key).encode("utf-8"), short_date)
    region_key = hmac_sha256(date_key, region)
    service_key = hmac_sha256(region_key, "s3")
    return hmac_sha256(service_key, "aws4_request")


def signed_headers(url: str, access_key: str, secret_key: str, region: str) -> dict[str, str]:
    parsed = urllib.parse.urlparse(url)
    now = dt.datetime.now(dt.timezone.utc)
    amz_date = now.strftime("%Y%m%dT%H%M%SZ")
    short_date = now.strftime("%Y%m%d")
    payload_hash = hashlib.sha256(b"").hexdigest()
    host = parsed.netloc.lower()
    canonical_headers = f"host:{host}\nx-amz-content-sha256:{payload_hash}\nx-amz-date:{amz_date}\n"
    signed = "host;x-amz-content-sha256;x-amz-date"
    canonical_request = "\n".join([
        "GET",
        parsed.path or "/",
        parsed.query,
        canonical_headers,
        signed,
        payload_hash,
    ])
    credential_scope = f"{short_date}/{region}/s3/aws4_request"
    string_to_sign = "\n".join([
        "AWS4-HMAC-SHA256",
        amz_date,
        credential_scope,
        hashlib.sha256(canonical_request.encode("utf-8")).hexdigest(),
    ])
    signature = hmac.new(signing_key(secret_key, short_date, region), string_to_sign.encode("utf-8"), hashlib.sha256).hexdigest()
    return {
        "Host": host,
        "X-Amz-Date": amz_date,
        "X-Amz-Content-Sha256": payload_hash,
        "Authorization": f"AWS4-HMAC-SHA256 Credential={access_key}/{credential_scope}, SignedHeaders={signed}, Signature={signature}",
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--credentials", required=True, type=Path)
    parser.add_argument("--object", required=True, dest="object_name")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    data = parse_credentials(args.credentials)
    endpoint = pick(data, "s3_endpoint", "Endpoint")
    bucket = pick(data, "s3_bucket", "Bucket")
    region = pick(data, "s3_region", "Region", required=False) or "auto"
    access_key = pick(data, "s3_access_key_id", "SecretId")
    secret_key = pick(data, "s3_secret_access_key", "SecretKey")
    path_style = bool_pick(data, "s3_path_style", "S3PathStyle")

    url = object_url(endpoint, bucket, args.object_name, path_style)
    req = urllib.request.Request(url, method="GET", headers=signed_headers(url, access_key, secret_key, region))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            payload = resp.read()
    except urllib.error.HTTPError as exc:
        raise SystemExit(f"download failed with HTTP {exc.code}") from exc
    args.output.write_bytes(payload)
    print(f"downloaded object={args.object_name} bytes={len(payload)} output={args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
