"""Build a reproducible few-shot block for each email in the evaluation dataset.

Prints one JSON object per email to stdout, or to --out:
    {"id": ..., "examples": [...], "few_shot_block": "..."}

The dataset holds personal transactions, and so does this output.
Keep --out outside the repository.
"""
import argparse
import json
import random
import sys

MAX_EXAMPLES = 10
SEED = 42


def norm(text):
    # same normalisation the evaluation uses to compare merchants
    return " ".join(str(text).lower().split())


def load_rows(path):
    with open(path, encoding="utf-8") as f:
        return [json.loads(line) for line in f if line.strip()]


def build_pool(rows):
    # identical example lines add nothing, so repeat visits to a merchant collapse into one
    unique = {}
    for row in rows:
        gold = row["gold"]
        if gold["is_receipt"] != 1:
            continue
        key = (norm(gold["merchant"]), gold["category"], gold["is_expense"])
        unique.setdefault(key, {
            "merchant": gold["merchant"],
            "category": gold["category"],
            "is_expense": gold["is_expense"],
        })

    # sorted so the sample does not depend on dataset order
    return [unique[key] for key in sorted(unique, key=str)]


def select_examples(row, pool, seed=SEED, limit=MAX_EXAMPLES):
    merchant = row["gold"]["merchant"]

    # non-receipts have no merchant, so nothing is excluded for them
    excluded = norm(merchant) if merchant else None
    candidates = [ex for ex in pool if norm(ex["merchant"]) != excluded]

    # a string seed is hashed with sha512, so it is stable across runs and PYTHONHASHSEED
    rng = random.Random(f"{seed}:{row['id']}")
    return rng.sample(candidates, min(limit, len(candidates)))


def build_few_shot_block(examples):
    # mirrors buildFewShotBlock in internal/email/ollama.go
    if not examples:
        return ""

    lines = ["Here are past transactions that were confirmed correct — use these as reference:\n"]
    for ex in examples:
        # go prints booleans as true/false with %v
        is_expense = "true" if ex["is_expense"] else "false"
        lines.append(f"  merchant: {ex['merchant']} → category: {ex['category']}, is_expense: {is_expense}\n")
    lines.append("\n")
    return "".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("dataset", help="path to scalar_llm_dataset.jsonl")
    parser.add_argument("--out", help="output jsonl path, defaults to stdout")
    parser.add_argument("--seed", type=int, default=SEED)
    parser.add_argument("--limit", type=int, default=MAX_EXAMPLES)
    args = parser.parse_args()

    rows = load_rows(args.dataset)
    pool = build_pool(rows)

    out = open(args.out, "w", encoding="utf-8", newline="\n") if args.out else sys.stdout
    for row in rows:
        examples = select_examples(row, pool, args.seed, args.limit)
        record = {"id": row["id"], "examples": examples, "few_shot_block": build_few_shot_block(examples)}
        out.write(json.dumps(record, ensure_ascii=False) + "\n")

    if args.out:
        out.close()


if __name__ == "__main__":
    main()
