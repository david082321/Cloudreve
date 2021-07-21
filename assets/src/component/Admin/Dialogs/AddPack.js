import React, { useState } from "react";
import DialogTitle from "@material-ui/core/DialogTitle";
import DialogContent from "@material-ui/core/DialogContent";
import DialogContentText from "@material-ui/core/DialogContentText";
import DialogActions from "@material-ui/core/DialogActions";
import Button from "@material-ui/core/Button";
import Dialog from "@material-ui/core/Dialog";
import InputLabel from "@material-ui/core/InputLabel";
import Input from "@material-ui/core/Input";
import FormHelperText from "@material-ui/core/FormHelperText";
import FormControl from "@material-ui/core/FormControl";
import SizeInput from "../Common/SizeInput";
import { makeStyles } from "@material-ui/core/styles";

const useStyles = makeStyles(() => ({
    formContainer: {
        margin: "8px 0 8px 0",
    },
}));

export default function AddPack({ open, onClose, onSubmit }) {
    const classes = useStyles();
    const [pack, setPack] = useState({
        name: "",
        size: "1073741824",
        time: "",
        price: "",
        score: "",
    });

    const handleChange = (name) => (event) => {
        setPack({
            ...pack,
            [name]: event.target.value,
        });
    };

    const submit = (e) => {
        e.preventDefault();
        const packCopy = { ...pack };
        packCopy.size = parseInt(packCopy.size);
        packCopy.time = parseInt(packCopy.time) * 86400;
        packCopy.price = parseInt(packCopy.price) * 100;
        packCopy.score = parseInt(packCopy.score);
        packCopy.id = new Date().valueOf();
        onSubmit(packCopy);
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            aria-labelledby="alert-dialog-title"
            aria-describedby="alert-dialog-description"
            maxWidth={"xs"}
        >
            <form onSubmit={submit}>
                <DialogTitle id="alert-dialog-title">添加容量包</DialogTitle>
                <DialogContent>
                    <DialogContentText id="alert-dialog-description">
                        <div className={classes.formContainer}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    名稱
                                </InputLabel>
                                <Input
                                    value={pack.name}
                                    onChange={handleChange("name")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    商品展示名稱
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.formContainer}>
                            <FormControl fullWidth>
                                <SizeInput
                                    value={pack.size}
                                    onChange={handleChange("size")}
                                    min={1}
                                    label={"大小"}
                                    max={9223372036854775807}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    容量包的大小
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.formContainer}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    有效期 (天)
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={pack.time}
                                    onChange={handleChange("time")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    每個容量包的有效期
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.formContainer}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    單價 (元)
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 0.01,
                                        step: 0.01,
                                    }}
                                    value={pack.price}
                                    onChange={handleChange("price")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    容量包的單價
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.formContainer}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    單價 (積分)
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 0,
                                        step: 1,
                                    }}
                                    value={pack.score}
                                    onChange={handleChange("score")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    使用積分購買時的價格，填寫為 0
                                    表示不能使用積分購買
                                </FormHelperText>
                            </FormControl>
                        </div>
                    </DialogContentText>
                </DialogContent>
                <DialogActions>
                    <Button onClick={onClose} color="default">
                        取消
                    </Button>
                    <Button type={"submit"} color="primary">
                        確定
                    </Button>
                </DialogActions>
            </form>
        </Dialog>
    );
}
