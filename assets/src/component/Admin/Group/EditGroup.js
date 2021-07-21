import React, { useCallback, useEffect, useState } from "react";
import { useParams } from "react-router";
import API from "../../../middleware/Api";
import { useDispatch } from "react-redux";
import { toggleSnackbar } from "../../../actions";
import GroupForm from "./GroupForm";

export default function EditGroupPreload() {
    const [group, setGroup] = useState({});

    const { id } = useParams();

    const dispatch = useDispatch();
    const ToggleSnackbar = useCallback(
        (vertical, horizontal, msg, color) =>
            dispatch(toggleSnackbar(vertical, horizontal, msg, color)),
        [dispatch]
    );

    useEffect(() => {
        setGroup({});
        API.get("/admin/group/" + id)
            .then((response) => {
                // 布林值轉換
                ["ShareEnabled", "WebDAVEnabled"].forEach((v) => {
                    response.data[v] = response.data[v] ? "true" : "false";
                });
                [
                    "archive_download",
                    "archive_task",
                    "one_time_download",
                    "share_download",
                    "aria2",
                ].forEach((v) => {
                    if (response.data.OptionsSerialized[v] !== undefined) {
                        response.data.OptionsSerialized[v] = response.data
                            .OptionsSerialized[v]
                            ? "true"
                            : "false";
                    }
                });

                // 整型轉換
                ["MaxStorage", "SpeedLimit"].forEach((v) => {
                    response.data[v] = response.data[v].toString();
                });
                ["compress_size", "decompress_size"].forEach((v) => {
                    if (response.data.OptionsSerialized[v] !== undefined) {
                        response.data.OptionsSerialized[
                            v
                        ] = response.data.OptionsSerialized[v].toString();
                    }
                });
                response.data.PolicyList = response.data.PolicyList[0];

                // JSON轉換
                if (
                    response.data.OptionsSerialized.aria2_options === undefined
                ) {
                    response.data.OptionsSerialized.aria2_options = "{}";
                } else {
                    try {
                        response.data.OptionsSerialized.aria2_options = JSON.stringify(
                            response.data.OptionsSerialized.aria2_options
                        );
                    } catch (e) {
                        ToggleSnackbar(
                            "top",
                            "right",
                            "Aria2 設定項格式錯誤",
                            "warning"
                        );
                        return;
                    }
                }
                setGroup(response.data);
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });
    }, [id]);

    return <div>{group.ID !== undefined && <GroupForm group={group} />}</div>;
}
