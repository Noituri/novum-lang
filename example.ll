; ModuleID = 'novumroot'
source_filename = "novumroot"

@strtmp = private unnamed_addr constant [13 x i8] c"Hello world!\00", align 1
@strtmp.1 = private unnamed_addr constant [15 x i8] c"Hello world 2!\00", align 1
@strtmp.2 = private unnamed_addr constant [4 x i8] c"%s\0A\00", align 1

define void @main() {
entry:
  call void @writeln(i8* getelementptr inbounds ([13 x i8], [13 x i8]* @strtmp, i32 0, i32 0))
  call void @writeln(i8* getelementptr inbounds ([15 x i8], [15 x i8]* @strtmp.1, i32 0, i32 0))
  %0 = call i1 @"binary_&&"(i1 true, i1 true)
  %1 = call i1 @"unary_!"(i1 %0)
  %2 = call i1 @"binary_&&"(i1 false, i1 %1)
  br i1 %2, label %then, label %else

then:                                             ; preds = %entry
  ret void

else:                                             ; preds = %entry
  br label %exit

exit:                                             ; preds = %else
  ret void
}

declare i32 @printf(i8* %msg, i8* %format)

define i1 @"binary_&&"(i1 %val_a, i1 %val_b) {
entry:
  %cmptmp = icmp eq i1 %val_a, %val_b
  br i1 %cmptmp, label %then, label %else

then:                                             ; preds = %entry
  ret i1 true

else:                                             ; preds = %entry
  br label %exit

exit:                                             ; preds = %else
  ret i1 false
}

define i1 @"unary_!"(i1 %a) {
entry:
  br i1 %a, label %then, label %else

then:                                             ; preds = %entry
  ret i1 false

else:                                             ; preds = %entry
  br label %exit

exit:                                             ; preds = %else
  ret i1 true
}

define void @writeln(i8* %msg) {
entry:
  %0 = call i32 @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @strtmp.2, i32 0, i32 0), i8* %msg)
  ret void
}
