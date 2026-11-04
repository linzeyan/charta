use std::fmt;

/// Why a chart could not be built.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Error {
    /// Width or height is not a positive, finite number.
    InvalidSize,
    /// The chart's data cannot be drawn; the message names the offending item.
    InvalidData(String),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::InvalidSize => f.write_str("chart width and height must be positive and finite"),
            Error::InvalidData(msg) => write!(f, "invalid chart data: {msg}"),
        }
    }
}

impl std::error::Error for Error {}
