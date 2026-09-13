#!/usr/bin/env python3
"""Static validator for the AgriPadi knowledge base.

Runs without external dependencies:
    python tools/validate_knowledge_base.py
"""
from __future__ import annotations
import csv
import datetime as dt
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
DATA = ROOT / "dataset"
ALLOWED_PESTS = {"penggerek batang", "walang sangit", "wereng batang cokelat"}
ALLOWED_STAGES = {"vegetatif", "generatif"}
ALLOWED_SEVERITIES = {"ringan", "sedang", "berat"}
ALLOWED_ROLES = {"identity", "damage", "severity_anchor", "supporting", "excluded_from_rule"}

def pipe_values(raw: str) -> set[str]:
    return {x.strip().lower() for x in raw.split("|") if x.strip()}


def normalize_pesticide_value(value: str) -> str:
    return " ".join(value.strip().casefold().replace(",", ".").split())

def fail(messages: list[str]) -> None:
    for message in messages:
        print("ERROR:", message)
    raise SystemExit(1)

def main() -> None:
    errors: list[str] = []
    with (DATA / "symptom_rule_simple.csv").open(encoding="utf-8-sig", newline="") as f:
        symptoms = list(csv.DictReader(f))
    names = set()
    symptom_by_name: dict[str, dict[str, str]] = {}
    by_pest = {p: [] for p in ALLOWED_PESTS}
    for i, row in enumerate(symptoms, 2):
        name = row["name"].strip()
        key = name.casefold()
        if not name or key in names:
            errors.append(f"symptom row {i}: empty or duplicate name {name!r}")
        names.add(key)
        symptom_by_name[key] = row
        pest = row["pest"].strip().lower()
        if pest not in ALLOWED_PESTS:
            errors.append(f"symptom row {i}: unknown pest {pest!r}")
            continue
        by_pest[pest].append(row)
        if not pipe_values(row["growth_stage"]).issubset(ALLOWED_STAGES):
            errors.append(f"symptom row {i}: invalid growth_stage")
        if not pipe_values(row["severity"]).issubset(ALLOWED_SEVERITIES):
            errors.append(f"symptom row {i}: invalid severity")
        if row["rule_role"].strip().lower() not in ALLOWED_ROLES:
            errors.append(f"symptom row {i}: invalid rule_role")
        try:
            weight=float(row["default_weight"])
            if not 0 < weight <= 1:
                raise ValueError
        except ValueError:
            errors.append(f"symptom row {i}: invalid default_weight")

    for pest, rows in by_pest.items():
        roles={r["rule_role"].strip().lower() for r in rows if r["recommended_for_rule"].strip().lower() in {"true","1","yes","ya"}}
        if "identity" not in roles or not ({"damage","severity_anchor"} & roles):
            errors.append(f"{pest}: must have identity and damage evidence")

    with (DATA / "rules.csv").open(encoding="utf-8-sig", newline="") as f:
        rules=list(csv.DictReader(f))
    rule_codes: set[str] = set()
    rule_combinations: set[tuple[str, str, str]] = set()
    for i,row in enumerate(rules,2):
        code = row["rule_code"].strip().casefold()
        pest = row["pest"].strip().lower()
        stage = row["growth_stage"].strip().lower()
        severity = row["severity"].strip().lower()
        if not code or code in rule_codes:
            errors.append(f"rule row {i}: empty or duplicate rule_code")
        rule_codes.add(code)
        combination = (pest, stage, severity)
        if combination in rule_combinations:
            errors.append(f"rule row {i}: duplicate pest-stage-severity combination {combination}")
        rule_combinations.add(combination)
        if pest not in ALLOWED_PESTS:
            errors.append(f"rule row {i}: unknown pest {pest!r}")
        if stage not in ALLOWED_STAGES:
            errors.append(f"rule row {i}: invalid growth_stage {stage!r}")
        if severity not in ALLOWED_SEVERITIES:
            errors.append(f"rule row {i}: invalid severity {severity!r}")

        selected=[x.strip().casefold() for x in row["symptoms"].split(";") if x.strip()]
        missing=[x for x in selected if x not in names]
        if missing:
            errors.append(f"rule row {i}: unknown symptoms {missing}")
        if len(selected) < 3:
            errors.append(f"rule row {i}: fewer than 3 symptoms")

        roles: set[str] = set()
        for symptom_name in selected:
            symptom = symptom_by_name.get(symptom_name)
            if not symptom:
                continue
            symptom_pest = symptom["pest"].strip().lower()
            symptom_stages = pipe_values(symptom["growth_stage"])
            symptom_severities = pipe_values(symptom["severity"])
            recommended = symptom["recommended_for_rule"].strip().lower() in {"true","1","yes","ya"}
            role = symptom["rule_role"].strip().lower()
            roles.add(role)
            if symptom_pest != pest:
                errors.append(f"rule row {i}: symptom {symptom_name!r} belongs to {symptom_pest}, not {pest}")
            if stage not in symptom_stages:
                errors.append(f"rule row {i}: symptom {symptom_name!r} is not valid for stage {stage}")
            if severity not in symptom_severities:
                errors.append(f"rule row {i}: symptom {symptom_name!r} is not valid for severity {severity}")
            if not recommended or role == "excluded_from_rule":
                errors.append(f"rule row {i}: symptom {symptom_name!r} is not recommended for rules")

        if "identity" not in roles or not ({"damage", "severity_anchor"} & roles):
            errors.append(f"rule row {i}: must contain pest identity and plant damage evidence")
        if pest=="walang sangit" and stage!="generatif":
            errors.append(f"rule row {i}: walang sangit must be generative")

    with (DATA / "pesticide.csv").open(encoding="utf-8-sig", newline="") as f:
        products=list(csv.DictReader(f))
    seen=set(); today=dt.date.today()
    identity_fields = (
        "product_name", "nama_komoditas", "target_pest", "ingredient_name",
        "ingredient_concentration", "ingredient_unit", "dose_value", "dose_unit",
    )
    for i,row in enumerate(products,2):
        key=tuple(normalize_pesticide_value(row[field]) for field in identity_fields)
        if key in seen:
            errors.append(f"pesticide row {i}: duplicate product-target-ingredient-dose record")
        seen.add(key)
        if row["nama_komoditas"].strip().lower()!="padi":
            errors.append(f"pesticide row {i}: commodity is not padi")
        if row["target_pest"].strip().lower() not in ALLOWED_PESTS:
            errors.append(f"pesticide row {i}: unknown target pest")
        try:
            reg=dt.date.fromisoformat(row["registered_at"])
            exp=dt.date.fromisoformat(row["expired_at"]) if row["expired_at"].strip() else None
            if exp and exp < reg:
                errors.append(f"pesticide row {i}: expiration before registration")
            if reg > today:
                errors.append(f"pesticide row {i}: registration date is in the future")
            if exp and exp < today:
                errors.append(f"pesticide row {i}: registration has expired")
        except ValueError:
            errors.append(f"pesticide row {i}: invalid date")

    if errors:
        fail(errors)
    print(f"PASS: {len(symptoms)} symptoms, {len(rules)} rules, {len(products)} unique pesticide rows")

if __name__ == "__main__":
    main()
