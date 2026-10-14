"""Independent piecewise evaluation of the published NIDDK formula, 60 digits.
Run with python3; compare values to calculate_test.go. No runtime dependency.
"""
from decimal import Decimal, localcontext
with localcontext() as context:
    context.prec = 60
    for sex, k, alpha, factor, values in [
        ("female", ".7", "-.241", "1.012", [".5", ".7", "1.4"]),
        ("male", ".9", "-.302", "1", [".5", ".9", "1.8"]),
    ]:
        for raw in values:
            ratio = Decimal(raw) / Decimal(k)
            exponent = Decimal(alpha) if ratio <= 1 else Decimal("-1.2")
            value = Decimal(142) * ratio**exponent * Decimal(".9938")**40 * Decimal(factor)
            print(sex, raw, value)
