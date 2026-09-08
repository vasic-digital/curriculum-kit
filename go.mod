module github.com/vasic-digital/curriculum-kit

go 1.26

// DELIBERATELY EMPTY REQUIRE SET — load-bearing, not incidental.
//
// 1. THE CONSUMERS ARE THE REASON.
//    This module is imported by application backends that already carry their
//    own database driver, their own web framework and their own logger. If it
//    dragged in a second one of any of those, every consumer would inherit
//    those lines in its own go.sum and its own supply-chain surface, and would
//    then have two opinions about the same job. Stdlib only.
//
// 2. IT IS A SEPARATE MODULE SO "PROJECT-NOT-AWARE" IS COMPILER-ENFORCED.
//    A pkg/ directory inside a consuming application could still import that
//    application's own domain types, and the reusability claim would rest on a
//    reviewer noticing. A separate module cannot: it does not require any
//    consumer, so no consumer-shaped symbol can appear here even by accident.
//    TestNoDependencies asserts the require set stays empty.
//
// 3. IT IS NOT internal/.
//    Go's internal/ is importable only from within its own module, by language
//    rule — the least reusable placement available.
