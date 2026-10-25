"""Calendar validity, numeric counters and JST day boundaries."""

import datetime as dt
import unittest

from calver import jst_date, next_tag, parse_tag


class CalVerTests(unittest.TestCase):
    def test_tag_validation(self):
        self.assertEqual(parse_tag("v2026.10.18.0"), (dt.date(2026, 10, 18), 0))
        self.assertEqual(parse_tag("v2024.02.29.12"), (dt.date(2024, 2, 29), 12))
        for invalid in ("2026.10.18.0", "v2026.1.18.0", "v2026.10.8.0", "v2026.10.18.01", "v2026.10.18.-1", "v2026.10.18", "v2026.02.29.0", "v2026.13.01.0", "v2026.10.18.0-rc1", "v2026.10.18.0\n"):
            with self.subTest(tag=invalid), self.assertRaises(ValueError):
                parse_tag(invalid)

    def test_same_day_counter_and_daily_reset(self):
        date = dt.date(2026, 10, 18)
        self.assertEqual(next_tag([], date), "v2026.10.18.0")
        self.assertEqual(next_tag(["v2026.10.18.0"], date), "v2026.10.18.1")
        self.assertEqual(next_tag(["v2026.10.18.9", "v2026.10.18.10", "v1.0.0"], date), "v2026.10.18.11")
        self.assertEqual(next_tag(["v2026.10.18.10"], dt.date(2026, 10, 19)), "v2026.10.19.0")
        with self.assertRaises(ValueError):
            next_tag(["v2026.10.19.0"], date)

    def test_jst_date_boundary(self):
        before = dt.datetime(2026, 10, 18, 14, 59, 59, tzinfo=dt.timezone.utc)
        after = dt.datetime(2026, 10, 18, 15, 0, 0, tzinfo=dt.timezone.utc)
        self.assertEqual(jst_date(before), dt.date(2026, 10, 18))
        self.assertEqual(jst_date(after), dt.date(2026, 10, 19))


if __name__ == "__main__":
    unittest.main()
