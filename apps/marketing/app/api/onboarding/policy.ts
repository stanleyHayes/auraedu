export interface PublicOnboardingFailure {
  status: 409 | 422 | 429 | 502;
  body: { code: string; message: string };
}

export function isValidIdempotencyKey(key: string | null): key is string {
  return key !== null && key.length >= 16 && key.length <= 128;
}

export function publicOnboardingFailure(upstreamStatus: number): PublicOnboardingFailure {
  if (upstreamStatus === 429) {
    return {
      status: 429,
      body: {
        code: "rate_limited",
        message: "Too many requests. Please wait a moment and try again.",
      },
    };
  }
  if (upstreamStatus === 409) {
    return {
      status: 409,
      body: {
        code: "conflict",
        message: "This request key was already used for different information.",
      },
    };
  }
  if (upstreamStatus === 422) {
    return {
      status: 422,
      body: {
        code: "submission_failed",
        message: "We could not accept the request. Please check the form and try again.",
      },
    };
  }
  return {
    status: 502,
    body: {
      code: "submission_failed",
      message: "We could not accept the request. Please check the form and try again.",
    },
  };
}
