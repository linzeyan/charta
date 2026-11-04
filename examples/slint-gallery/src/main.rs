//! Every chart charta can draw, in a Slint window. Run with `make run-slint`.

use slint::ComponentHandle;

fn main() -> Result<(), slint::PlatformError> {
    charta_slint_gallery::gallery()?.run()
}
