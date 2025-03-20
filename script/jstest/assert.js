'use strict';

const assert = {
    _isSameValue(a, b) {
        if (a === b) {
            // Handle +/-0 vs. -/+0
            return a !== 0 || 1 / a === 1 / b;
        }

        // Handle NaN vs. NaN
        return a !== a && b !== b;
    },

    _toString(value) {
        try {
            if (value === 0 && 1 / value === -Infinity) {
                return '-0';
            }

            return String(value);
        } catch (err) {
            if (err.name === 'TypeError') {
                return Object.prototype.toString.call(value);
            }

            throw err;
        }
    },

    sameValue(actual, expected, message) {
        if (assert._isSameValue(actual, expected)) {
            return;
        }
        if (message === undefined) {
            message = '';
        } else {
            message += ' ';
        }

        message += 'Expected SameValue(«' + assert._toString(actual) + '», «' + assert._toString(expected) + '») to be true';

        throw new Error(message);
    },

    throws(f, ctor, message) {
        if (message === undefined) {
            message = '';
        } else {
            message += ' ';
        }
        try {
            f();
        } catch (e) {
            if (e.constructor !== ctor) {
                throw new Error(message + "Wrong exception type was thrown: " + e.constructor.name);
            }
            return;
        }
        throw new Error(message + "No exception was thrown");
    },

    throwsNodeError(f, ctor, code, message) {
        if (message === undefined) {
            message = '';
        } else {
            message += ' ';
        }
        try {
            f();
        } catch (e) {
            if (e.constructor !== ctor) {
                throw new Error(message + "Wrong exception type was thrown: " + e.constructor.name);
            }
            if (e.code !== code) {
                throw new Error(message + "Wrong exception code was thrown: " + e.code);
            }
            return;
        }
        throw new Error(message + "No exception was thrown");
    }
}

module.exports = assert;